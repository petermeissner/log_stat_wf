let autoRefreshInterval = null;
let currentRefreshFrequency = 10000; // Default 10 seconds
let currentData = []; // Store current table data
let sortColumn = null;
let sortDirection = 'asc'; // 'asc' or 'desc'
let levelChart = null; // Chart.js instance

// Color mapping for log levels
const levelColors = {
    'FATAL': '#8B0000',    // Dark red
    'ERROR': '#FF4444',    // Red
    'WARN': '#FFA500',     // Orange
    'WARNING': '#FFA500',  // Orange (alternative)
    'INFO': '#4CAF50',     // Green
    'DEBUG': '#2196F3',    // Blue
    'TRACE': '#9C27B0'     // Purple
};

// Initialize form handlers and load initial data on page load
document.addEventListener('DOMContentLoaded', function() {
    // Initialize chart
    initializeChart();

    // Handle filter form submission
    document.getElementById('filterForm').addEventListener('submit', function(e) {
        e.preventDefault();
        loadStats();
    });

    // Handle refresh frequency change
    document.getElementById('refreshFrequency').addEventListener('change', function() {
        currentRefreshFrequency = parseInt(this.value);
        if (autoRefreshInterval) {
            // Restart auto-refresh with new frequency
            stopAutoRefresh();
            startAutoRefresh();
        }
    });

    // Handle auto-refresh checkbox
    document.getElementById('autoRefreshCheckbox').addEventListener('change', function() {
        if (this.checked) {
            startAutoRefresh();
        } else {
            stopAutoRefresh();
        }
    });

    // Handle manual refresh button
    document.getElementById('manualRefreshBtn').addEventListener('click', function() {
        loadStats();
    });

    // Handle table header clicks for sorting
    document.getElementById('statsTable').addEventListener('click', function(e) {
        if (e.target.classList.contains('sortable') || e.target.parentElement.classList.contains('sortable')) {
            const th = e.target.classList.contains('sortable') ? e.target : e.target.parentElement;
            const column = th.getAttribute('data-column');
            sortTable(column);
        }
    });

    // Load stats on initial page load
    loadStats();
});

function initializeChart() {
    const ctx = document.getElementById('levelChart').getContext('2d');
    levelChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [{
                label: 'Message Count by Level',
                data: [],
                backgroundColor: [],
                borderColor: '#ddd',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            indexAxis: 'x',
            scales: {
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'Count'
                    }
                }
            },
            plugins: {
                legend: {
                    display: false
                }
            }
        }
    });
}

function updateChart(data) {
    // Aggregate counts by level
    const levelCounts = {};
    
    data.forEach(stat => {
        const level = stat.Level || 'UNKNOWN';
        levelCounts[level] = (levelCounts[level] || 0) + stat.N;
    });
    
    // Sort levels by count (descending)
    const sortedLevels = Object.keys(levelCounts).sort((a, b) => levelCounts[b] - levelCounts[a]);
    
    // Prepare chart data
    const labels = sortedLevels;
    const counts = sortedLevels.map(level => levelCounts[level]);
    const colors = sortedLevels.map(level => levelColors[level] || '#999');
    
    // Update chart
    levelChart.data.labels = labels;
    levelChart.data.datasets[0].data = counts;
    levelChart.data.datasets[0].backgroundColor = colors;
    levelChart.update();
}

function startAutoRefresh() {
    if (autoRefreshInterval) {
        clearInterval(autoRefreshInterval);
    }
    autoRefreshInterval = setInterval(loadStats, currentRefreshFrequency);
}

function stopAutoRefresh() {
    if (autoRefreshInterval) {
        clearInterval(autoRefreshInterval);
        autoRefreshInterval = null;
    }
}

function buildQueryParams() {
    const minTs = document.getElementById('minTs').value;
    const maxTs = document.getElementById('maxTs').value;
    
    let params = new URLSearchParams();
    
    if (minTs) {
        // Convert datetime-local to RFC3339 format (add Z for UTC)
        params.append('min_ts', new Date(minTs).toISOString());
    }
    
    if (maxTs) {
        // Convert datetime-local to RFC3339 format (add Z for UTC)
        params.append('max_ts', new Date(maxTs).toISOString());
    }
    
    return params;
}

function sortTable(column) {
    // Toggle sort direction if clicking same column, otherwise sort ascending
    if (sortColumn === column) {
        sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
        sortColumn = column;
        sortDirection = 'asc';
    }
    
    // Sort the data
    currentData.sort((a, b) => {
        let aVal = a[column];
        let bVal = b[column];
        
        // Handle Rate column (calculated, not in data)
        if (column === 'Rate') {
            aVal = calculateRate(a.N, a.BucketDuration_S);
            bVal = calculateRate(b.N, b.BucketDuration_S);
            aVal = parseFloat(aVal);
            bVal = parseFloat(bVal);
        } else if (column === 'N' || column === 'BucketDuration_S') {
            // Numeric columns
            aVal = parseFloat(aVal) || 0;
            bVal = parseFloat(bVal) || 0;
        } else {
            // String columns - case insensitive
            aVal = String(aVal || '').toLowerCase();
            bVal = String(bVal || '').toLowerCase();
        }
        
        let comparison = 0;
        if (aVal < bVal) {
            comparison = -1;
        } else if (aVal > bVal) {
            comparison = 1;
        }
        
        return sortDirection === 'asc' ? comparison : -comparison;
    });
    
    // Update sort indicators
    document.querySelectorAll('th.sortable').forEach(th => {
        th.classList.remove('sort-asc', 'sort-desc');
    });
    
    const activeHeader = document.querySelector(`th[data-column="${column}"]`);
    if (activeHeader) {
        activeHeader.classList.add(sortDirection === 'asc' ? 'sort-asc' : 'sort-desc');
    }
    
    // Re-render the table
    renderTableData(currentData);
}

function loadStats() {
    const loading = document.getElementById('loading');
    const error = document.getElementById('error');
    const table = document.getElementById('statsTable');
    const tbody = document.getElementById('statsBody');
    
    loading.style.display = 'block';
    error.style.display = 'none';
    tbody.innerHTML = '';
    
    const params = buildQueryParams();
    const url = '/api/stats' + (params.toString() ? '?' + params.toString() : '');
    
    fetch(url)
        .then(response => {
            if (!response.ok) throw new Error('Failed to fetch stats');
            return response.json();
        })
        .then(data => {
            loading.style.display = 'none';
            
            // Store data for sorting
            currentData = data || [];
            
            if (!currentData || currentData.length === 0) {
                tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 20px;">No statistics available for the selected time range</td></tr>';
                table.style.display = 'table';
                // Update chart with empty data
                updateChart([]);
                return;
            }
            
            // Update chart
            updateChart(currentData);
            
            // Apply current sort if one is active
            if (sortColumn) {
                currentData.sort((a, b) => {
                    let aVal = a[sortColumn];
                    let bVal = b[sortColumn];
                    
                    if (sortColumn === 'Rate') {
                        aVal = parseFloat(calculateRate(a.N, a.BucketDuration_S));
                        bVal = parseFloat(calculateRate(b.N, b.BucketDuration_S));
                    } else if (sortColumn === 'N' || sortColumn === 'BucketDuration_S') {
                        aVal = parseFloat(aVal) || 0;
                        bVal = parseFloat(bVal) || 0;
                    } else {
                        aVal = String(aVal || '').toLowerCase();
                        bVal = String(bVal || '').toLowerCase();
                    }
                    
                    let comparison = 0;
                    if (aVal < bVal) comparison = -1;
                    else if (aVal > bVal) comparison = 1;
                    
                    return sortDirection === 'asc' ? comparison : -comparison;
                });
            }
            
            renderTableData(currentData);
            table.style.display = 'table';
        })
        .catch(err => {
            loading.style.display = 'none';
            error.textContent = 'Error: ' + err.message;
            error.style.display = 'block';
        });
}

function renderTableData(data) {
    const tbody = document.getElementById('statsBody');
    tbody.innerHTML = '';
    
    data.forEach(stat => {
        const row = document.createElement('tr');
        const rate = calculateRate(stat.N, stat.BucketDuration_S);
        const cells = [
            escapeHtml(stat.HostName || ''),
            escapeHtml(stat.Logger || ''),
            escapeHtml(stat.Level || ''),
            stat.N || 0,
            formatTimestamp(stat.BucketTS),
            formatTimestamp(stat.FirstSeenTS),
            stat.BucketDuration_S || 0,
            rate
        ];
        row.innerHTML = cells.map(cell => '<td>' + cell + '</td>').join('');
        tbody.appendChild(row);
    });
}

function calculateRate(count, durationSeconds) {
    if (durationSeconds === 0) return '0.00';
    return (count / durationSeconds * 3600).toFixed(2);
}

function formatTimestamp(timestamp) {
    try {
        return new Date(timestamp).toLocaleString();
    } catch (e) {
        return timestamp;
    }
}

function escapeHtml(text) {
    const map = {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'};
    return String(text).replace(/[&<>"']/g, m => map[m]);
}


