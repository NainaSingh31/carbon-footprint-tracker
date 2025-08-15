package main

import (
	"encoding/json"

	"net/http"
	"time"
	"strings"
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	CreatedAt time.Time `json:"created_at"`
}

type Activity struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Category   string    `json:"category"` // transport, energy, food, shopping, other
	Type       string    `json:"type"`     // e.g., car, bus, electricity, vegetarian_day, etc.
	Quantity   float64   `json:"quantity"` // numeric input for the type (e.g., distance_km, kWh, spend)
	Unit       string    `json:"unit"`
	Meta       string    `json:"meta"`     // raw JSON string for any extra fields
	EmissionKg float64   `json:"emission_kg"`
	Date       time.Time `json:"date"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateActivityDTO struct {
	Category string   `json:"category" binding:"required"`
	Type     string   `json:"type" binding:"required"`
	Quantity float64  `json:"quantity"`
	Unit     string   `json:"unit"`
	Meta     gin.H    `json:"meta"`
	Date     string   `json:"date"` // ISO date YYYY-MM-DD (optional); defaults to today
}

type SummaryResponse struct {
	From        string             `json:"from"`
	To          string             `json:"to"`
	TotalKg     float64            `json:"total_kg"`
	ByCategory  map[string]float64 `json:"by_category"`
	ByDay       []DailyPoint       `json:"by_day"`
}

type DailyPoint struct {
	Date string  `json:"date"`
	Kg   float64 `json:"kg"`
}

type App struct {
	DB *gorm.DB
}

func main() {
	db, err := gorm.Open(sqlite.Open("app.db"), &gorm.Config{})
	if err != nil { panic(err) }
	db.AutoMigrate(&User{}, &Activity{})

	app := &App{DB: db}

	r := gin.Default()

	// CORS for local dev (frontend on 5173 or a file://)
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET","POST","DELETE","OPTIONS"},
		AllowHeaders: []string{"Origin","Content-Type","Accept"},
	}))

	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	// seed a default user if none exists (no auth for simplicity)
	var count int64
	db.Model(&User{}).Count(&count)
	if count == 0 {
		db.Create(&User{Name: "Demo User", Location: "Earth"})
	}

	r.GET("/api/activities", app.listActivities)
	r.POST("/api/activities", app.createActivity)
	r.DELETE("/api/activities/:id", app.deleteActivity)
	r.GET("/api/summary", app.summary)

	r.Run(":8080")
}

func (a *App) listActivities(c *gin.Context) {
	var items []Activity
	if err := a.DB.Order("date asc, id asc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (a *App) createActivity(c *gin.Context) {
	var dto CreateActivityDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// parse date
	var d time.Time
	if strings.TrimSpace(dto.Date) == "" {
		d = time.Now()
	} else {
		var err error
		d, err = time.Parse("2006-01-02", dto.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date, use YYYY-MM-DD"})
			return
		}
	}

	// compute emission
	emission := computeEmission(dto.Category, dto.Type, dto.Quantity, dto.Unit, dto.Meta)

	metaJSON := "{}"
	if dto.Meta != nil {
		if b, err := jsonMarshal(dto.Meta); err == nil {
			metaJSON = string(b)
		}
	}

	item := Activity{
		Category: dto.Category,
		Type: dto.Type,
		Quantity: dto.Quantity,
		Unit: dto.Unit,
		Meta: metaJSON,
		EmissionKg: emission,
		Date: d,
	}
	if err := a.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (a *App) deleteActivity(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := a.DB.Delete(&Activity{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (a *App) summary(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	var from, to time.Time
	var err error

	if strings.TrimSpace(fromStr) == "" {
		from = time.Now().AddDate(0, 0, -29) // last 30 days
	} else {
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"}); return }
	}

	if strings.TrimSpace(toStr) == "" {
		to = time.Now()
	} else {
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"}); return }
	}

	var items []Activity
	if err := a.DB.Where("date BETWEEN ? AND ?", from, to).Order("date asc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	byCat := map[string]float64{}
	total := 0.0
	byDay := map[string]float64{}

	for _, it := range items {
		total += it.EmissionKg
		byCat[it.Category] += it.EmissionKg
		key := it.Date.Format("2006-01-02")
		byDay[key] += it.EmissionKg
	}

	// fill daily points across the range
	points := []DailyPoint{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		points = append(points, DailyPoint{Date: key, Kg: byDay[key]})
	}

	c.JSON(http.StatusOK, SummaryResponse{
		From: from.Format("2006-01-02"),
		To: to.Format("2006-01-02"),
		TotalKg: round2(total),
		ByCategory: roundMap(byCat),
		ByDay: points,
	})
}

// --------------------
// Emission calculator
// --------------------

func computeEmission(category, typ string, qty float64, unit string, meta map[string]any) float64 {
	// Baseline factors (illustrative averages, kg CO2e per unit):
	// Transport per km:
	carPerKm := 0.192  // average car
	busPerKm := 0.105
	trainPerKm := 0.041
	bikePerKm := 0.0
	airPerKm := 0.255  // short/medium haul rough average

	// Energy:
	kWhFactor := 0.7   // kg/kWh (adjust for your grid)
	lpgKgFactor := 3.0 // per kg LPG burned ~3 kg CO2e (simplified)

	// Food (per day):
	meatHeavy := 7.0
	vegetarian := 3.0
	vegan := 2.0

	// Shopping (per 1000 currency units):
	shoppingFactorPerThousand := 1.5 // kg CO2e per 1000 units of currency (very rough)

	category = strings.ToLower(category)
	typ = strings.ToLower(typ)
	unit = strings.ToLower(unit)

	switch category {
	case "transport":
		switch typ {
		case "car":
			// qty = distance km
			return round2(qty * carPerKm)
		case "bus":
			return round2(qty * busPerKm)
		case "train":
			return round2(qty * trainPerKm)
		case "bike", "walk":
			return 0.0
		case "flight":
			return round2(qty * airPerKm)
		}
	case "energy":
		switch typ {
		case "electricity":
			// qty = kWh
			return round2(qty * kWhFactor)
		case "lpg":
			// qty = kg of LPG
			return round2(qty * lpgKgFactor)
		}
	case "food":
		switch typ {
		case "meat_heavy_day":
			return meatHeavy
		case "vegetarian_day":
			return vegetarian
		case "vegan_day":
			return vegan
		}
	case "shopping":
		// qty = spend
		return round2((qty / 1000.0) * shoppingFactorPerThousand)
	case "other":
		// if user provides a direct emission value in kg
		if unit == "kgco2e" || unit == "kg" {
			return round2(qty)
		}
	}
	return 0.0
}

// ---------------
// Helper helpers
// ---------------

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

type dummyJSON struct{} // just to require encoding/json
var _ = dummyJSON{}

func round2(v float64) float64 {
	return mathRound(v*100.0) / 100.0
}

func roundMap(m map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for k, v := range m {
		out[k] = round2(v)
	}
	return out
}

// minimal math round
func mathRound(x float64) float64 {
	if x < 0 { return float64(int64(x-0.5)) }
	return float64(int64(x+0.5))
}
