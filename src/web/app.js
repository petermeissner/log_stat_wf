function loadStats() {
    const loading = document.getElementById('loading');
    const error = document.getElementById('error');
    const table = document.getElementById('statsTable');
    const tbody = document.getElementById('statsBody');
    
    loading.style.display = 'block';
    error.style.display = 'none';
    table.style.display = 'none';
    tbody.innerHTML = '';
    
    fetch('/api/stats')
        .then(response => {
            if (!response.ok) throw new Error('Failed to fetch stats');
            return response.json();
        })
        .then(data => {
            loading.style.display = 'none';
            if (!data || data.length === 0) {
                error.textContent = 'No statistics available yet';
                error.style.display = 'block';
                return;
            }
            data.forEach(stat => {
                const row = document.createElement('tr');
                const cells = [
                    escapeHtml(stat.HostName),
                    escapeHtml(stat.Logger),
                    escapeHtml(stat.Level),
                    stat.N,
                    new Date(stat.TS_Start).toLocaleString(),
                    stat.TS_Interval_S
                ];
                row.innerHTML = cells.map(cell => '<td>' + cell + '</td>').join('');
                tbody.appendChild(row);
            });
            table.style.display = 'table';
        })
        .catch(err => {
            loading.style.display = 'none';
            error.textContent = 'Error: ' + err.message;
            error.style.display = 'block';
        });
}

function escapeHtml(text) {
    const map = {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'};
    return String(text).replace(/[&<>"']/g, m => map[m]);
}

// Load stats on page load and refresh every 5 seconds
loadStats();
setInterval(loadStats, 5000);
