let autoRefreshInterval = null;
let currentRefreshFrequency = 10000; // Default 10 seconds
let currentData = []; // Store current table data
let filteredData = []; // Store filtered data for client-side filtering
let sortColumn = null;
let sortDirection = 'asc'; // 'asc' or 'desc'
let levelChart = null; // ECharts instance
let timeSeriesChart = null; // Time-series chart instance
let memoryChart = null; // Memory chart instance
let currentViewMode = 'detailed'; // 'detailed' or 'aggregated'
let currentPage = 'dashboard'; // Current active page
let quickFilterText = ''; // Client-side filter text
let quickFilterLevel = ''; // Client-side filter level
let availableLoggers = []; // List of all unique loggers
let selectedLoggers = []; // Currently selected loggers for chart
let selectedLevels = ['INFO']; // Currently selected levels for time-series chart (default INFO)
let loggerColors = {}; // Color mapping for loggers

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
    // Initialize charts
    initializeChart();
    initializeTimeSeriesChart();
    initializeMemoryChart();

    // Setup navigation
    setupNavigation();

    // Setup quick filter handlers
    setupQuickFilters();
    
    // Setup logger selection handlers
    setupLoggerSelection();
    
    // Setup level selection handlers
    setupLevelSelection();

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
    const chartDom = document.getElementById('levelChart');
    levelChart = echarts.init(chartDom);
    
    const option = {
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow'
            }
        },
        legend: {
            data: ['FATAL', 'ERROR', 'WARN', 'WARNING', 'INFO', 'DEBUG', 'TRACE'],
            type: 'scroll',
            bottom: 0
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '15%',
            containLabel: true
        },
        xAxis: {
            type: 'category',
            data: []
        },
        yAxis: {
            type: 'value',
            name: 'Count'
        },
        series: []
    };
    
    levelChart.setOption(option);
    
    // Handle window resize
    window.addEventListener('resize', function() {
        if (levelChart) levelChart.resize();
    });
}

function initializeTimeSeriesChart() {
    const chartDom = document.getElementById('timeSeriesChart');
    timeSeriesChart = echarts.init(chartDom);
    
    const option = {
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'cross'
            }
        },
        legend: {
            data: [],
            type: 'scroll',
            bottom: 0
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '15%',
            containLabel: true
        },
        toolbox: {
            feature: {
                dataZoom: {
                    yAxisIndex: 'none'
                },
                restore: {},
                saveAsImage: {}
            }
        },
        dataZoom: [
            {
                type: 'inside',
                start: 0,
                end: 100
            },
            {
                start: 0,
                end: 100
            }
        ],
        xAxis: {
            type: 'category',
            boundaryGap: false,
            data: []
        },
        yAxis: {
            type: 'value',
            name: 'Message Count'
        },
        series: []
    };
    
    timeSeriesChart.setOption(option);
    
    // Handle window resize
    window.addEventListener('resize', function() {
        if (timeSeriesChart) timeSeriesChart.resize();
    });
}

function setupNavigation() {
    // Handle navigation tab clicks
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.addEventListener('click', function() {
            const pageName = this.getAttribute('data-page');
            navigateToPage(pageName);
        });
    });
}

function setupQuickFilters() {
    const searchInput = document.getElementById('quickSearchInput');
    const levelFilter = document.getElementById('quickLevelFilter');
    const clearButton = document.getElementById('clearQuickFilter');
    
    // Debounce search input to avoid filtering on every keystroke
    let searchTimeout;
    searchInput.addEventListener('input', function() {
        clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => {
            quickFilterText = this.value.toLowerCase();
            applyQuickFilter();
        }, 300); // 300ms debounce
    });
    
    // Level filter - apply immediately
    levelFilter.addEventListener('change', function() {
        quickFilterLevel = this.value;
        applyQuickFilter();
    });
    
    // Clear quick filters
    clearButton.addEventListener('click', function() {
        searchInput.value = '';
        levelFilter.value = '';
        quickFilterText = '';
        quickFilterLevel = '';
        applyQuickFilter();
    });
}

function setupLoggerSelection() {
    const loggerSelect = document.getElementById('loggerSelect');
    const topBtn = document.getElementById('selectTopLoggers');
    const allBtn = document.getElementById('selectAllLoggers');
    const clearBtn = document.getElementById('clearLoggers');
    
    // Handle logger selection changes
    loggerSelect.addEventListener('change', function() {
        selectedLoggers = Array.from(this.selectedOptions).map(opt => opt.value);
        updateTimeSeriesChart();
    });
    
    // Select top 5 loggers by volume
    topBtn.addEventListener('click', function() {
        if (availableLoggers.length === 0) return;
        
        // Calculate total volume per logger from filtered data
        const loggerVolumes = {};
        filteredData.forEach(stat => {
            const logger = stat.Logger || 'Unknown';
            loggerVolumes[logger] = (loggerVolumes[logger] || 0) + (stat.N || stat.TotalCount || 0);
        });
        
        // Sort and take top 5
        const top5 = Object.entries(loggerVolumes)
            .sort((a, b) => b[1] - a[1])
            .slice(0, 5)
            .map(entry => entry[0]);
        
        // Select in dropdown
        Array.from(loggerSelect.options).forEach(opt => {
            opt.selected = top5.includes(opt.value);
        });
        
        selectedLoggers = top5;
        updateTimeSeriesChart();
    });
    
    // Select all loggers
    allBtn.addEventListener('click', function() {
        Array.from(loggerSelect.options).forEach(opt => opt.selected = true);
        selectedLoggers = availableLoggers;
        updateTimeSeriesChart();
    });
    
    // Clear selection
    clearBtn.addEventListener('click', function() {
        loggerSelect.selectedIndex = -1;
        selectedLoggers = [];
        updateTimeSeriesChart();
    });
}

function setupLevelSelection() {
    const levelSelect = document.getElementById('levelSelect');
    const allBtn = document.getElementById('selectAllLevels');
    const clearBtn = document.getElementById('clearLevels');
    
    // Handle level selection changes
    levelSelect.addEventListener('change', function() {
        selectedLevels = Array.from(this.selectedOptions).map(opt => opt.value);
        updateTimeSeriesChart();
    });
    
    // Select all levels
    allBtn.addEventListener('click', function() {
        Array.from(levelSelect.options).forEach(opt => opt.selected = true);
        selectedLevels = Array.from(levelSelect.options).map(opt => opt.value);
        updateTimeSeriesChart();
    });
    
    // Clear selection
    clearBtn.addEventListener('click', function() {
        levelSelect.selectedIndex = -1;
        selectedLevels = [];
        updateTimeSeriesChart();
    });
}

function applyQuickFilter() {
    // Start with all current data
    filteredData = [...currentData];
    
    // Apply text search filter
    if (quickFilterText) {
        filteredData = filteredData.filter(stat => {
            const searchableText = [
                stat.HostName || '',
                stat.Logger || '',
                stat.Level || ''
            ].join(' ').toLowerCase();
            return searchableText.includes(quickFilterText);
        });
    }
    
    // Apply level filter
    if (quickFilterLevel) {
        filteredData = filteredData.filter(stat => stat.Level === quickFilterLevel);
    }
    
    // Update filter status
    const statusEl = document.getElementById('filterStatus');
    const total = currentData.length;
    const filtered = filteredData.length;
    statusEl.textContent = `(${filtered}/${total})`;
    statusEl.style.display = 'inline';
    
    // Update charts with filtered data
    updateChart(filteredData);
    updateLoggerList(filteredData);
    updateTimeSeriesChart();
    
    // Apply sorting if active
    if (sortColumn) {
        sortTableData(filteredData);
    }
    
    // Re-render table with filtered data
    renderTableData(filteredData);
    
    // Show empty message if no results
    if (filteredData.length === 0 && currentData.length > 0) {
        const tbody = document.getElementById('statsBody');
        const colspan = currentViewMode === 'aggregated' ? '6' : '8';
        tbody.innerHTML = `<tr><td colspan="${colspan}" style="text-align: center; padding: 20px;">No records match the current filter</td></tr>`;
    }
}

function navigateToPage(pageName) {
    // Update active tab
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.classList.remove('active');
    });
    document.querySelector(`.nav-tab[data-page="${pageName}"]`).classList.add('active');
    
    // Show/hide pages
    document.querySelectorAll('.page-content').forEach(page => {
        page.classList.remove('active');
    });
    document.getElementById(`page-${pageName}`).classList.add('active');
    
    currentPage = pageName;
    
    // Load page-specific data
    if (pageName === 'dashboard') {
        loadStats();
    } else if (pageName === 'system') {
        loadSystemInfo();
    } else if (pageName === 'database') {
        loadDatabaseInfo();
    } else if (pageName === 'stream') {
        initializeStreamPage();
        // Trigger resize after page is visible to ensure chart has correct dimensions
        setTimeout(() => {
            window.dispatchEvent(new Event('resize'));
        }, 150);
    }
}

function initializeMemoryChart() {
    const chartDom = document.getElementById('memoryChart');
    memoryChart = echarts.init(chartDom);
    
    const option = {
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'cross'
            }
        },
        legend: {
            data: ['Heap Allocated', 'RSS']
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '3%',
            containLabel: true
        },
        xAxis: {
            type: 'category',
            boundaryGap: false,
            data: []
        },
        yAxis: {
            type: 'value',
            name: 'Memory (MB)'
        },
        series: [
            {
                name: 'Heap Allocated',
                type: 'line',
                smooth: true,
                data: [],
                areaStyle: {
                    opacity: 0.3
                },
                lineStyle: {
                    color: '#2196F3'
                },
                itemStyle: {
                    color: '#2196F3'
                }
            },
            {
                name: 'RSS',
                type: 'line',
                smooth: true,
                data: [],
                areaStyle: {
                    opacity: 0.3
                },
                lineStyle: {
                    color: '#4CAF50'
                },
                itemStyle: {
                    color: '#4CAF50'
                }
            }
        ]
    };
    
    memoryChart.setOption(option);
    
    // Handle window resize
    window.addEventListener('resize', function() {
        if (memoryChart) memoryChart.resize();
    });
}

function updateChart(data) {
    if (!levelChart) return;
    
    // Aggregate data by timestamp and level
    const timeSeriesData = {};
    const timestamps = new Set();
    const levels = ['FATAL', 'ERROR', 'WARN', 'WARNING', 'INFO', 'DEBUG', 'TRACE'];
    
    data.forEach(stat => {
        const timestamp = stat.BucketTS;
        const level = stat.Level || 'UNKNOWN';
        const count = stat.TotalCount || stat.N || 0;
        
        timestamps.add(timestamp);
        
        if (!timeSeriesData[level]) {
            timeSeriesData[level] = {};
        }
        
        timeSeriesData[level][timestamp] = (timeSeriesData[level][timestamp] || 0) + count;
    });
    
    // Sort timestamps
    const sortedTimestamps = Array.from(timestamps).sort();
    const formattedTimestamps = sortedTimestamps.map(ts => formatTimestamp(ts));
    
    // Create series for each level as stacked bars
    const series = levels.map(level => {
        const color = levelColors[level] || '#999';
        const data = sortedTimestamps.map(ts => timeSeriesData[level]?.[ts] || 0);
        
        return {
            name: level,
            type: 'bar',
            stack: 'Total',
            emphasis: {
                focus: 'series'
            },
            data: data,
            itemStyle: {
                color: color
            }
        };
    }).filter(s => s.data.some(v => v > 0)); // Only include levels with data
    
    // Update chart (notMerge: true to replace data instead of merging)
    levelChart.setOption({
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow'
            }
        },
        legend: {
            data: series.map(s => s.name),
            type: 'scroll',
            bottom: 0
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '15%',
            containLabel: true
        },
        xAxis: {
            type: 'category',
            data: formattedTimestamps
        },
        yAxis: {
            type: 'value',
            name: 'Count'
        },
        series: series
    }, true); // notMerge: true
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
    
    // Sort the filtered data (not the original data)
    sortTableData(filteredData);
    
    // Update sort indicators
    document.querySelectorAll('th.sortable').forEach(th => {
        th.classList.remove('sort-asc', 'sort-desc');
    });
    
    const activeHeader = document.querySelector(`th[data-column="${column}"]`);
    if (activeHeader) {
        activeHeader.classList.add(sortDirection === 'asc' ? 'sort-asc' : 'sort-desc');
    }
    
    // Re-render the table with filtered and sorted data
    renderTableData(filteredData);
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
            
            // Store data for sorting and filtering
            currentData = data || [];
            filteredData = [...currentData]; // Initialize filtered data
            
            // Reset quick filters when new data is loaded
            quickFilterText = '';
            quickFilterLevel = '';
            document.getElementById('quickSearchInput').value = '';
            document.getElementById('quickLevelFilter').value = '';
            
            // Update filter status with initial count
            const statusEl = document.getElementById('filterStatus');
            statusEl.textContent = `(${currentData.length}/${currentData.length})`;
            statusEl.style.display = 'inline';
            
            if (!currentData || currentData.length === 0) {
                const colspan = viewMode === 'aggregated' ? '6' : '8';
                tbody.innerHTML = `<tr><td colspan="${colspan}" style="text-align: center; padding: 20px;">No statistics available for the selected filters</td></tr>`;
                table.style.display = 'table';
                updateChart([]);
                return;
            }
            
            // Update charts
            updateChart(currentData);
            updateLoggerList(currentData);
            updateTimeSeriesChart();
            
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

function loadSystemInfo() {
    // Load system information from stats that include memory data
    const memoryDiv = document.getElementById('memoryStats');
    const runtimeDiv = document.getElementById('runtimeStats');
    
    memoryDiv.innerHTML = '<div class="loading">Loading memory statistics...</div>';
    runtimeDiv.innerHTML = '<div class="loading">Loading runtime information...</div>';
    
    // Fetch recent stats with memory data
    fetch('/api/query/recent?hours=1&max_results=10&include_memory=true')
        .then(response => response.json())
        .then(data => {
            if (data && data.length > 0) {
                // Get the most recent stat with memory info
                const recent = data[0];
                
                // Display memory stats
                memoryDiv.innerHTML = `
                    <div class="stat-item">
                        <span class="stat-label">RSS Memory:</span>
                        <span class="stat-value">${formatBytes(recent.RSS_Bytes || 0)}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Virtual Memory:</span>
                        <span class="stat-value">${formatBytes(recent.VMS_Bytes || 0)}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Heap Allocated:</span>
                        <span class="stat-value">${formatBytes(recent.HeapAlloc_Bytes || 0)}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Heap System:</span>
                        <span class="stat-value">${formatBytes(recent.HeapSys_Bytes || 0)}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Stack:</span>
                        <span class="stat-value">${formatBytes(recent.Stack_Bytes || 0)}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Goroutines:</span>
                        <span class="stat-value">${recent.Goroutines || 0}</span>
                    </div>
                `;
                
                // Display runtime stats
                runtimeDiv.innerHTML = `
                    <div class="stat-item">
                        <span class="stat-label">Host:</span>
                        <span class="stat-value">${escapeHtml(recent.HostName || 'N/A')}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Last Update:</span>
                        <span class="stat-value">${formatTimestamp(recent.BucketTS)}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Total Records:</span>
                        <span class="stat-value">${data.length}</span>
                    </div>
                `;
                
                // Update memory chart with historical data
                updateMemoryChart(data);
            } else {
                memoryDiv.innerHTML = '<div class="error">No memory data available</div>';
                runtimeDiv.innerHTML = '<div class="error">No runtime data available</div>';
            }
        })
        .catch(err => {
            memoryDiv.innerHTML = `<div class="error">Error loading memory stats: ${err.message}</div>`;
            runtimeDiv.innerHTML = `<div class="error">Error loading runtime stats: ${err.message}</div>`;
        });
}

function loadDatabaseInfo() {
    const dbStatsDiv = document.getElementById('dbStats');
    const storageDiv = document.getElementById('storageStats');
    const activityDiv = document.getElementById('recentActivity');
    
    dbStatsDiv.innerHTML = '<div class="loading">Loading database statistics...</div>';
    storageDiv.innerHTML = '<div class="loading">Loading storage information...</div>';
    activityDiv.innerHTML = '<div class="loading">Loading recent activity...</div>';
    
    // Fetch aggregated stats to get database overview
    fetch('/api/query/aggregated?max_results=100&include_db=true')
        .then(response => response.json())
        .then(data => {
            if (data && data.length > 0) {
                // Calculate database stats
                let totalRecords = 0;
                let uniqueLevels = new Set();
                let uniqueHosts = new Set();
                let oldestTimestamp = null;
                let newestTimestamp = null;
                
                data.forEach(stat => {
                    totalRecords += stat.TotalCount || 0;
                    uniqueLevels.add(stat.Level);
                    uniqueHosts.add(stat.HostName);
                    
                    const ts = new Date(stat.BucketTS);
                    if (!oldestTimestamp || ts < oldestTimestamp) {
                        oldestTimestamp = ts;
                    }
                    if (!newestTimestamp || ts > newestTimestamp) {
                        newestTimestamp = ts;
                    }
                });
                
                dbStatsDiv.innerHTML = `
                    <div class="stat-item">
                        <span class="stat-label">Total Records:</span>
                        <span class="stat-value">${totalRecords.toLocaleString()}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Unique Levels:</span>
                        <span class="stat-value">${uniqueLevels.size}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Unique Hosts:</span>
                        <span class="stat-value">${uniqueHosts.size}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Data Buckets:</span>
                        <span class="stat-value">${data.length}</span>
                    </div>
                `;
                
                storageDiv.innerHTML = `
                    <div class="stat-item">
                        <span class="stat-label">Oldest Record:</span>
                        <span class="stat-value">${oldestTimestamp ? formatTimestamp(oldestTimestamp) : 'N/A'}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Newest Record:</span>
                        <span class="stat-value">${newestTimestamp ? formatTimestamp(newestTimestamp) : 'N/A'}</span>
                    </div>
                    <div class="stat-item">
                        <span class="stat-label">Data Retention:</span>
                        <span class="stat-value">30 days</span>
                    </div>
                `;
                
                // Show recent activity by level
                const recentByLevel = {};
                data.slice(0, 10).forEach(stat => {
                    const level = stat.Level || 'UNKNOWN';
                    if (!recentByLevel[level]) {
                        recentByLevel[level] = 0;
                    }
                    recentByLevel[level] += stat.TotalCount || 0;
                });
                
                let activityHtml = '<div class="activity-list">';
                Object.entries(recentByLevel)
                    .sort((a, b) => b[1] - a[1])
                    .forEach(([level, count]) => {
                        const color = levelColors[level] || '#999';
                        activityHtml += `
                            <div class="activity-item">
                                <span class="activity-level" style="background-color: ${color}">${level}</span>
                                <span class="activity-count">${count.toLocaleString()} messages</span>
                            </div>
                        `;
                    });
                activityHtml += '</div>';
                activityDiv.innerHTML = activityHtml;
            } else {
                dbStatsDiv.innerHTML = '<div class="error">No database data available</div>';
                storageDiv.innerHTML = '<div class="error">No storage data available</div>';
                activityDiv.innerHTML = '<div class="error">No activity data available</div>';
            }
        })
        .catch(err => {
            dbStatsDiv.innerHTML = `<div class="error">Error loading database stats: ${err.message}</div>`;
            storageDiv.innerHTML = `<div class="error">Error loading storage info: ${err.message}</div>`;
            activityDiv.innerHTML = `<div class="error">Error loading activity: ${err.message}</div>`;
        });
}

function updateMemoryChart(data) {
    if (!memoryChart) return;
    
    // Extract memory data from stats (most recent first)
    const sortedData = [...data].sort((a, b) => 
        new Date(b.BucketTS) - new Date(a.BucketTS)
    ).slice(0, 20).reverse(); // Show last 20 data points
    
    const labels = sortedData.map(stat => formatTimestamp(stat.BucketTS));
    const heapData = sortedData.map(stat => (stat.HeapAlloc_Bytes || 0) / 1024 / 1024);
    const rssData = sortedData.map(stat => (stat.RSS_Bytes || 0) / 1024 / 1024);
    
    memoryChart.setOption({
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'cross'
            }
        },
        legend: {
            data: ['Heap Allocated', 'RSS']
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '3%',
            containLabel: true
        },
        xAxis: {
            type: 'category',
            boundaryGap: false,
            data: labels
        },
        yAxis: {
            type: 'value',
            name: 'Memory (MB)'
        },
        series: [
            {
                name: 'Heap Allocated',
                type: 'line',
                smooth: true,
                data: heapData,
                areaStyle: {
                    opacity: 0.3
                },
                lineStyle: {
                    color: '#2196F3'
                },
                itemStyle: {
                    color: '#2196F3'
                }
            },
            {
                name: 'RSS',
                type: 'line',
                smooth: true,
                data: rssData,
                areaStyle: {
                    opacity: 0.3
                },
                lineStyle: {
                    color: '#4CAF50'
                },
                itemStyle: {
                    color: '#4CAF50'
                }
            }
        ]
    }, true); // notMerge: true
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

function updateLoggerList(data) {
    // Extract unique loggers from data
    const loggersSet = new Set();
    data.forEach(stat => {
        const logger = stat.Logger || 'Unknown';
        loggersSet.add(logger);
    });
    
    availableLoggers = Array.from(loggersSet).sort();
    
    // Update dropdown
    const select = document.getElementById('loggerSelect');
    const previouslySelected = selectedLoggers;
    
    select.innerHTML = '';
    if (availableLoggers.length === 0) {
        select.innerHTML = '<option disabled>No loggers available</option>';
        return;
    }
    
    availableLoggers.forEach(logger => {
        const option = document.createElement('option');
        option.value = logger;
        option.textContent = logger;
        // Restore previous selection if logger still exists
        if (previouslySelected.includes(logger)) {
            option.selected = true;
        }
        select.appendChild(option);
    });
    
    // Update selected loggers list
    selectedLoggers = previouslySelected.filter(l => availableLoggers.includes(l));
}

function updateTimeSeriesChart() {
    if (!timeSeriesChart) return;
    
    if (selectedLoggers.length === 0) {
        // Show empty state
        timeSeriesChart.setOption({
            tooltip: {
                trigger: 'axis',
                axisPointer: {
                    type: 'cross'
                }
            },
            legend: {
                data: [],
                type: 'scroll',
                bottom: 0
            },
            grid: {
                left: '3%',
                right: '4%',
                bottom: '15%',
                containLabel: true
            },
            xAxis: {
                type: 'category',
                boundaryGap: false,
                data: []
            },
            yAxis: {
                type: 'value',
                name: 'Message Count'
            },
            series: []
        }, true); // notMerge: true
        return;
    }
    
    // Aggregate data by logger and time
    const timeSeriesData = {};
    const timestamps = new Set();
    
    filteredData.forEach(stat => {
        const logger = stat.Logger || 'Unknown';
        if (!selectedLoggers.includes(logger)) return;
        
        // Filter by selected levels
        const level = stat.Level || 'UNKNOWN';
        if (selectedLevels.length > 0 && !selectedLevels.includes(level)) return;
        
        const timestamp = stat.BucketTS;
        timestamps.add(timestamp);
        
        if (!timeSeriesData[logger]) {
            timeSeriesData[logger] = {};
        }
        
        const count = stat.N || stat.TotalCount || 0;
        timeSeriesData[logger][timestamp] = (timeSeriesData[logger][timestamp] || 0) + count;
    });
    
    // Sort timestamps
    const sortedTimestamps = Array.from(timestamps).sort();
    const formattedTimestamps = sortedTimestamps.map(ts => formatTimestamp(ts));
    
    // Create series for each logger
    const series = selectedLoggers.map((logger, index) => {
        const color = getLoggerColor(logger, index);
        const data = sortedTimestamps.map(ts => timeSeriesData[logger]?.[ts] || 0);
        
        return {
            name: logger,
            type: 'line',
            step: 'end', // Use step line
            data: data,
            lineStyle: {
                color: color,
                width: 2
            },
            itemStyle: {
                color: color
            },
            emphasis: {
                focus: 'series'
            }
        };
    });
    
    // Update chart (notMerge: true to replace data instead of merging)
    timeSeriesChart.setOption({
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'cross'
            }
        },
        legend: {
            data: selectedLoggers,
            type: 'scroll',
            bottom: 0
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '15%',
            containLabel: true
        },
        toolbox: {
            feature: {
                dataZoom: {
                    yAxisIndex: 'none'
                },
                restore: {},
                saveAsImage: {}
            }
        },
        dataZoom: [
            {
                type: 'inside',
                start: 0,
                end: 100
            },
            {
                start: 0,
                end: 100
            }
        ],
        xAxis: {
            type: 'category',
            boundaryGap: false,
            data: formattedTimestamps
        },
        yAxis: {
            type: 'value',
            name: 'Message Count'
        },
        series: series
    }, true); // notMerge: true
}

function getLoggerColor(logger, index) {
    // Return cached color if exists
    if (loggerColors[logger]) {
        return loggerColors[logger];
    }
    
    // Generate distinct colors using HSL
    const hue = (index * 137.5) % 360; // Golden angle for better distribution
    const saturation = 65 + (index % 3) * 10;
    const lightness = 45 + (index % 2) * 10;
    const color = `hsl(${hue}, ${saturation}%, ${lightness}%)`;
    
    loggerColors[logger] = color;
    return color;
}

// Stream page initialization flag
let streamPageInitialized = false;

function initializeStreamPage() {
    // Only initialize once
    if (streamPageInitialized) {
        return;
    }
    streamPageInitialized = true;
    
    // Initialize stream functionality (defined in stream.js)
    if (typeof initializeStreamUI === 'function') {
        initializeStreamUI();
    }
}
