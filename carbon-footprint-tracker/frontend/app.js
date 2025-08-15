const API = 'http://localhost:8080/api';

const typesByCategory = {
  transport: ['car','bus','train','bike','walk','flight'],
  energy: ['electricity','lpg'],
  food: ['meat_heavy_day','vegetarian_day','vegan_day'],
  shopping: ['spend'],
  other: ['custom_kgco2e']
};

const defaultUnits = {
  car: 'km', bus: 'km', train: 'km', bike: 'km', walk: 'km', flight: 'km',
  electricity: 'kWh', lpg: 'kg',
  meat_heavy_day: 'day', vegetarian_day: 'day', vegan_day: 'day',
  spend: 'currency',
  custom_kgco2e: 'kgco2e'
};

const categoryEl = document.getElementById('category');
const typeEl = document.getElementById('type');
const quantityEl = document.getElementById('quantity');
const unitEl = document.getElementById('unit');
const dateEl = document.getElementById('date');
const form = document.getElementById('activity-form');
const formMsg = document.getElementById('form-msg');
const totalKgEl = document.getElementById('total-kg');
const tbody = document.querySelector('#activities-table tbody');

let pieChart, lineChart;

function setTypeOptions() {
  const cat = categoryEl.value;
  typeEl.innerHTML = '';
  typesByCategory[cat].forEach(t => {
    const opt = document.createElement('option');
    opt.value = t;
    opt.textContent = t;
    typeEl.appendChild(opt);
  });
  unitEl.value = defaultUnits[typeEl.value] || '';
}
categoryEl.addEventListener('change', setTypeOptions);
typeEl.addEventListener('change', () => unitEl.value = defaultUnits[typeEl.value] || '');
setTypeOptions();

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  const cat = categoryEl.value;
  let t = typeEl.value;
  let qty = parseFloat(quantityEl.value || '0');
  let unit = unitEl.value;
  let date = dateEl.value;

  // map frontend types to backend expectations
  if (cat === 'shopping' && t === 'spend') t = 'shopping'; // backend expects category=shopping
  if (cat === 'other' && t === 'custom_kgco2e') t = 'custom'; // backend uses 'other' category

  const payload = {
    category: cat,
    type: (cat === 'shopping') ? 'shopping' : ((cat === 'other') ? 'custom' : t),
    quantity: qty,
    unit: unit,
    date: date
  };

  try {
    const res = await fetch(API + '/activities', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify(payload)
    });
    if (!res.ok) throw new Error(await res.text());
    await loadAll();
    formMsg.textContent = 'Saved ✓';
    form.reset();
    setTypeOptions();
    setTimeout(() => formMsg.textContent = '', 1500);
  } catch (err) {
    console.error(err);
    formMsg.textContent = 'Failed to save';
  }
});

async function loadAll() {
  await Promise.all([loadActivities(), loadSummary()]);
}

async function loadActivities() {
  const res = await fetch(API + '/activities');
  const items = await res.json();
  tbody.innerHTML = '';
  items.forEach(row => {
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td>${row.date?.slice(0,10)}</td>
      <td><span class="badge">${row.category}</span></td>
      <td>${row.type}</td>
      <td>${row.quantity}</td>
      <td>${row.unit || ''}</td>
      <td>${row.emission_kg}</td>
      <td><button data-id="${row.id}" class="delete-btn">Delete</button></td>
    `;
    tbody.appendChild(tr);
  });
  document.querySelectorAll('.delete-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.getAttribute('data-id');
      await fetch(API + '/activities/' + id, { method: 'DELETE' });
      loadAll();
    });
  });
}

async function loadSummary() {
  const res = await fetch(API + '/summary');
  const data = await res.json();
  totalKgEl.textContent = data.total_kg.toFixed(2) + ' kg';

  // Pie by category
  const labels = Object.keys(data.by_category);
  const values = Object.values(data.by_category);
  if (pieChart) pieChart.destroy();
  pieChart = new Chart(document.getElementById('pieByCategory'), {
    type: 'pie',
    data: { labels, datasets: [{ data: values }] },
    options: { plugins: { legend: { position: 'bottom' } } }
  });

  // Line by day
  const days = data.by_day.map(p => p.date);
  const vals = data.by_day.map(p => p.kg);
  if (lineChart) lineChart.destroy();
  lineChart = new Chart(document.getElementById('lineByDay'), {
    type: 'line',
    data: { labels: days, datasets: [{ label: 'kg CO2e', data: vals, tension: 0.3 }] },
    options: { scales: { y: { beginAtZero: true } } }
  });
}

// set default date to today
dateEl.valueAsDate = new Date();

loadAll();
