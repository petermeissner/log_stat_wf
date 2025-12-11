let autoRefreshInterval = null;
let currentRefreshFrequency = 10000; // Default 10 seconds
let currentData = []; // Store current table data
let sortColumn = null;
let sortDirection = 'asc'; // 'asc' or 'desc'
let levelChart = null; // Chart.js instance
let currentViewMode = 'detailed'; // 'detailed' or 'aggregated'

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

    // Handle view mode change
    document.getElementById('viewMode').addEventListener('change', function() {
        currentViewMode = this.value;
        loadStats();
    });

    // Handle clear filters button
    document.getElementById('clearFiltersBtn').addEventListener('click', function() {
        document.getElementById('filterForm').reset();
        document.getElementById('includeMemory').checked = true;
        document.getElementById('includeDB').checked = true;
        document.getElementById('maxResults').value = '1000'; // Set default
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

    // Set default max results if not set
    const maxResultsInput = document.getElementById('maxResults');
    if (!maxResultsInput.value) {
        maxResultsInput.value = '1000';
    }

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
        const count = stat.TotalCount || stat.N || 0; // Handle both detailed and aggregated
        levelCounts[level] = (levelCounts[level] || 0) + count;
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

function updateTableHeadersForDetailed() {
    const thead = document.getElementById('statsTableHead');
    thead.innerHTML = `
        <tr>
            <th class="sortable" data-column="HostName">Host<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="Logger">Logger<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="Level">Level<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="N">Count<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="BucketTS">Bucket Start<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="FirstSeenTS">First Seen<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="BucketDuration_S">Duration (s)<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="Rate">Rate (msg/hr)<span class="sort-indicator"></span></th>
        </tr>
    `;
}

function updateTableHeadersForAggregated() {
    const thead = document.getElementById('statsTableHead');
    thead.innerHTML = `
        <tr>
            <th class="sortable" data-column="HostName">Host<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="Level">Level<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="TotalCount">Total Count<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="LoggerCount">Logger Count<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="BucketTS">Bucket Start<span class="sort-indicator"></span></th>
            <th class="sortable" data-column="FirstSeenTS">First Seen<span class="sort-indicator"></span></th>
        </tr>
    `;
}

function sortTableData(data) {
    if (!sortColumn) return;
    
    data.sort((a, b) => {
        let aVal = a[sortColumn];
        let bVal = b[sortColumn];
        
        if (sortColumn === 'Rate') {
            aVal = parseFloat(calculateRate(a.N, a.BucketDuration_S));
            bVal = parseFloat(calculateRate(b.N, b.BucketDuration_S));
        } else if (sortColumn === 'N' || sortColumn === 'BucketDuration_S' || sortColumn === 'TotalCount' || sortColumn === 'LoggerCount') {
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

function renderDetailedTableData(data) {
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

function renderAggregatedTableData(data) {
    const tbody = document.getElementById('statsBody');
    tbody.innerHTML = '';
    
    data.forEach(stat => {
        const row = document.createElement('tr');
        const cells = [
            escapeHtml(stat.HostName || ''),
            escapeHtml(stat.Level || ''),
            stat.TotalCount || 0,
            stat.LoggerCount || 0,
            formatTimestamp(stat.BucketTS),
            formatTimestamp(stat.FirstSeenTS)
        ];
        row.innerHTML = cells.map(cell => '<td>' + cell + '</td>').join('');
        tbody.appendChild(row);
    });
}

function renderTableData(data) {
    if (currentViewMode === 'aggregated') {
        renderAggregatedTableData(data);
    } else {
        renderDetailedTableData(data);
    }
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
    const viewMode = document.getElementById('viewMode').value;
    const level = document.getElementById('level').value;
    const loggerRegex = document.getElementById('loggerRegex').value;
    const minTs = document.getElementById('minTs').value;
    const maxTs = document.getElementById('maxTs').value;
    const maxResults = document.getElementById('maxResults').value;
    const includeMemory = document.getElementById('includeMemory').checked;
    const includeDB = document.getElementById('includeDB').checked;
    
    let params = new URLSearchParams();
    
    if (level) {
        params.append('level', level);
    }
    
    if (loggerRegex) {
        params.append('logger_regex', loggerRegex);
    }
    
    if (minTs) {
        params.append('start_time', new Date(minTs).toISOString());
    }
    
    if (maxTs) {
        params.append('end_time', new Date(maxTs).toISOString());
    }
    
    // Always set max_results with default of 1000 if not specified
    const limit = maxResults || '1000';
    params.append('max_results', limit);
    
    params.append('include_memory', includeMemory);
    params.append('include_db', includeDB);
    
    return { params, viewMode };
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
    sortTableData(currentData);
    
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
    const tableTitle = document.getElementById('tableTitle');
    
    loading.style.display = 'block';
    error.style.display = 'none';
    tbody.innerHTML = '';
    
    const { params, viewMode } = buildQueryParams();
    
    // Choose API endpoint based on view mode
    let url;
    if (viewMode === 'aggregated') {
        url = '/api/query/aggregated?' + params.toString();
        tableTitle.textContent = 'Aggregated Log Statistics (by Level)';
        updateTableHeadersForAggregated();
    } else {
        url = '/api/query/stats?' + params.toString();
        tableTitle.textContent = 'Log Statistics (Detailed)';
        updateTableHeadersForDetailed();
    }
    
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
                const colspan = viewMode === 'aggregated' ? '5' : '8';
                tbody.innerHTML = `<tr><td colspan="${colspan}" style="text-align: center; padding: 20px;">No statistics available for the selected filters</td></tr>`;
                table.style.display = 'table';
                updateChart([]);
                return;
            }
            
            // Update chart
            updateChart(currentData);
            
            // Apply current sort if one is active
            if (sortColumn) {
                sortTableData(currentData);
            }
            
            if (viewMode === 'aggregated') {
                renderAggregatedTableData(currentData);
            } else {
                renderDetailedTableData(currentData);
            }
            table.style.display = 'table';
        })
        .catch(err => {
            loading.style.display = 'none';
            error.textContent = 'Error: ' + err.message;
            error.style.display = 'block';
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


