// WebSocket Log Stream Client
class LogStreamClient {
    constructor() {
        this.ws = null;
        this.url = null;
        this.connected = false;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.reconnectDelay = 1000;
        this.messageCallback = null;
        this.statsCallback = null;
        this.errorCallback = null;
        this.statusCallback = null;
        this.connectCallback = null; // Callback when connection is established
        this.lastSubscription = null; // Store last subscription for reconnection
    }

    connect(url) {
        this.url = url;
        this.updateStatus('connecting', 'Connecting...');

        try {
            this.ws = new WebSocket(url);
            
            this.ws.onopen = () => {
                this.connected = true;
                this.reconnectAttempts = 0;
                this.updateStatus('connected', 'Connected');
                console.log('WebSocket connected');
                
                // Call connect callback if set
                if (this.connectCallback) {
                    this.connectCallback();
                }
            };

            this.ws.onmessage = (event) => {
                try {
                    const message = JSON.parse(event.data);
                    this.handleMessage(message);
                } catch (error) {
                    console.error('Failed to parse message:', error);
                }
            };

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                if (this.errorCallback) {
                    this.errorCallback('Connection error');
                }
            };

            this.ws.onclose = () => {
                this.connected = false;
                this.updateStatus('disconnected', 'Disconnected');
                console.log('WebSocket closed');
                this.attemptReconnect();
            };
        } catch (error) {
            console.error('Failed to create WebSocket:', error);
            this.updateStatus('disconnected', 'Connection failed');
            if (this.errorCallback) {
                this.errorCallback('Failed to create connection');
            }
        }
    }

    disconnect() {
        this.reconnectAttempts = this.maxReconnectAttempts; // Prevent reconnection
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.connected = false;
        this.updateStatus('disconnected', 'Disconnected');
    }

    attemptReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.log('Max reconnect attempts reached');
            if (this.errorCallback) {
                this.errorCallback('Connection lost. Click Connect to retry.');
            }
            return;
        }

        this.reconnectAttempts++;
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
        console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
        
        this.updateStatus('connecting', `Reconnecting (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);

        setTimeout(() => {
            if (!this.connected && this.url) {
                this.connect(this.url);
            }
        }, delay);
    }

    subscribe(filters) {
        if (!this.connected || !this.ws) {
            console.error('Not connected');
            return false;
        }

        const message = {
            action: 'subscribe',
            data: filters
        };

        // Store subscription for reconnection
        this.lastSubscription = message;

        try {
            this.ws.send(JSON.stringify(message));
            console.log('Subscription sent:', filters);
            return true;
        } catch (error) {
            console.error('Failed to send subscription:', error);
            return false;
        }
    }

    updateSubscription(filters) {
        if (!this.connected || !this.ws) {
            console.error('Not connected');
            return false;
        }

        const message = {
            action: 'update',
            data: filters
        };

        // Store subscription for reconnection (as subscribe type)
        this.lastSubscription = {
            action: 'subscribe',
            data: filters
        };

        try {
            this.ws.send(JSON.stringify(message));
            console.log('Subscription updated:', filters);
            return true;
        } catch (error) {
            console.error('Failed to update subscription:', error);
            return false;
        }
    }

    ping() {
        if (!this.connected || !this.ws) {
            return false;
        }

        const message = { action: 'ping' };
        try {
            this.ws.send(JSON.stringify(message));
            return true;
        } catch (error) {
            console.error('Failed to send ping:', error);
            return false;
        }
    }

    // Resend last subscription (used after reconnection)
    resendLastSubscription() {
        if (this.lastSubscription && this.connected && this.ws) {
            console.log('Resending last subscription after reconnect:', this.lastSubscription);
            try {
                this.ws.send(JSON.stringify(this.lastSubscription));
                return true;
            } catch (error) {
                console.error('Failed to resend subscription:', error);
                return false;
            }
        }
        return false;
    }

    requestStats() {
        if (!this.connected || !this.ws) {
            return false;
        }

        const message = { action: 'stats' };
        try {
            this.ws.send(JSON.stringify(message));
            return true;
        } catch (error) {
            console.error('Failed to request stats:', error);
            return false;
        }
    }

    handleMessage(message) {
        switch (message.type) {
            case 'log':
                if (this.messageCallback) {
                    this.messageCallback(message.data);
                }
                break;
            
            case 'batch':
                if (this.messageCallback && message.data.messages) {
                    message.data.messages.forEach(msg => this.messageCallback(msg));
                }
                break;
            
            case 'stats':
                if (this.statsCallback) {
                    this.statsCallback(message.data);
                }
                break;
            
            case 'error':
                console.error('Server error:', message.data);
                if (this.errorCallback) {
                    this.errorCallback(message.data.message);
                }
                break;
            
            case 'ack':
                console.log('Server ack:', message.data);
                break;
            
            case 'pong':
                console.log('Server pong:', message.data);
                break;
            
            default:
                console.warn('Unknown message type:', message.type);
        }
    }

    updateStatus(status, text) {
        if (this.statusCallback) {
            this.statusCallback(status, text);
        }
    }

    onMessage(callback) {
        this.messageCallback = callback;
    }

    onStats(callback) {
        this.statsCallback = callback;
    }

    onError(callback) {
        this.errorCallback = callback;
    }

    onStatus(callback) {
        this.statusCallback = callback;
    }

    onConnect(callback) {
        this.connectCallback = callback;
    }

    isConnected() {
        return this.connected;
    }
}

// Global state
let client = new LogStreamClient();
let messages = [];
let messageBuffer = [];
let chart = null;
let chartData = new Map(); // timestamp -> {DEBUG: 0, INFO: 0, ...}
let updateTimer = null;
let statsTimer = null;
let maxMessages = 10000;
let updateInterval = 250;
let chartBucketSize = 20; // seconds
let chartWindow = 300; // seconds

// Main initialization function - called from app.js when stream page is shown
function initializeStreamUI() {
    initializeChart();
    setupCollapsibleSections();
    loadFilterSettings(); // Load saved filter settings from localStorage
    loadSettings();
    
    // Resize chart after a short delay to ensure container has correct dimensions
    setTimeout(() => {
        if (chart) {
            chart.resize();
        }
    }, 100);
    
    // Auto-connect after a short delay
    setTimeout(() => {
        const connectBtn = document.querySelector('#page-stream .controls-bar button');
        if (connectBtn && connectBtn.textContent === 'Connect') {
            toggleConnection();
        }
    }, 500);
}

function initializeChart() {
    const chartDom = document.getElementById('frequency-chart');
    chart = echarts.init(chartDom);

    const option = {
        animation: false, // Disable all animations for real-time performance
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow'
            }
        },
        legend: {
            data: ['DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL'],
            bottom: 0
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '10%',
            top: '5%',
            containLabel: true
        },
        xAxis: {
            type: 'category',
            data: [],
            axisLabel: {
                rotate: 45,
                formatter: (value) => {
                    const date = new Date(value);
                    return date.toLocaleTimeString();
                }
            }
        },
        yAxis: {
            type: 'value',
            name: 'Messages'
        },
        series: [
            { name: 'DEBUG', type: 'bar', stack: 'total', data: [], itemStyle: { color: '#2196F3' } },
            { name: 'INFO', type: 'bar', stack: 'total', data: [], itemStyle: { color: '#4CAF50' } },
            { name: 'WARN', type: 'bar', stack: 'total', data: [], itemStyle: { color: '#FFA500' } },
            { name: 'ERROR', type: 'bar', stack: 'total', data: [], itemStyle: { color: '#FF4444' } },
            { name: 'FATAL', type: 'bar', stack: 'total', data: [], itemStyle: { color: '#8B0000' } }
        ]
    };

    chart.setOption(option, true);
    
    // Resize chart on window resize
    window.addEventListener('resize', () => {
        chart.resize();
    });
}

function setupCollapsibleSections() {
    document.querySelectorAll('.filter-section h3.collapsible').forEach(header => {
        header.addEventListener('click', () => {
            header.classList.toggle('collapsed');
            header.nextElementSibling.classList.toggle('collapsed');
        });
    });
}

function loadSettings() {
    maxMessages = parseInt(document.getElementById('max-messages').value);
    updateInterval = parseInt(document.getElementById('update-interval').value);
    chartBucketSize = parseInt(document.getElementById('chart-bucket').value);
    chartWindow = parseInt(document.getElementById('chart-window').value);
}

function saveFilterSettings() {
    const settings = {
        // Log levels
        levelDebug: document.getElementById('level-debug').checked,
        levelInfo: document.getElementById('level-info').checked,
        levelWarn: document.getElementById('level-warn').checked,
        levelError: document.getElementById('level-error').checked,
        levelFatal: document.getElementById('level-fatal').checked,
        
        // Patterns
        hostPatterns: document.getElementById('host-patterns').value,
        loggerPatterns: document.getElementById('logger-patterns').value,
        
        // Advanced filters
        messageContains: document.getElementById('message-contains').value,
        messageExcludes: document.getElementById('message-excludes').value,
        messageRegex: document.getElementById('message-regex').value,
        stackTraceMode: document.getElementById('stack-trace-mode').value,
        stackInclude: document.getElementById('stack-include').value,
        stackExclude: document.getElementById('stack-exclude').value,
        
        // Performance
        rateLimit: document.getElementById('rate-limit').value,
        batchTimeout: document.getElementById('batch-timeout').value,
        
        // Display
        chartWindow: document.getElementById('chart-window').value,
        chartBucket: document.getElementById('chart-bucket').value,
        maxMessages: document.getElementById('max-messages').value,
        updateInterval: document.getElementById('update-interval').value
    };
    
    localStorage.setItem('streamFilterSettings', JSON.stringify(settings));
}

function loadFilterSettings() {
    const saved = localStorage.getItem('streamFilterSettings');
    if (!saved) return;
    
    try {
        const settings = JSON.parse(saved);
        
        // Log levels
        if (settings.levelDebug !== undefined) document.getElementById('level-debug').checked = settings.levelDebug;
        if (settings.levelInfo !== undefined) document.getElementById('level-info').checked = settings.levelInfo;
        if (settings.levelWarn !== undefined) document.getElementById('level-warn').checked = settings.levelWarn;
        if (settings.levelError !== undefined) document.getElementById('level-error').checked = settings.levelError;
        if (settings.levelFatal !== undefined) document.getElementById('level-fatal').checked = settings.levelFatal;
        
        // Patterns
        if (settings.hostPatterns !== undefined) document.getElementById('host-patterns').value = settings.hostPatterns;
        if (settings.loggerPatterns !== undefined) document.getElementById('logger-patterns').value = settings.loggerPatterns;
        
        // Advanced filters
        if (settings.messageContains !== undefined) document.getElementById('message-contains').value = settings.messageContains;
        if (settings.messageExcludes !== undefined) document.getElementById('message-excludes').value = settings.messageExcludes;
        if (settings.messageRegex !== undefined) document.getElementById('message-regex').value = settings.messageRegex;
        if (settings.stackTraceMode !== undefined) document.getElementById('stack-trace-mode').value = settings.stackTraceMode;
        if (settings.stackInclude !== undefined) document.getElementById('stack-include').value = settings.stackInclude;
        if (settings.stackExclude !== undefined) document.getElementById('stack-exclude').value = settings.stackExclude;
        
        // Performance
        if (settings.rateLimit !== undefined) document.getElementById('rate-limit').value = settings.rateLimit;
        if (settings.batchTimeout !== undefined) document.getElementById('batch-timeout').value = settings.batchTimeout;
        
        // Display
        if (settings.chartWindow !== undefined) document.getElementById('chart-window').value = settings.chartWindow;
        if (settings.chartBucket !== undefined) document.getElementById('chart-bucket').value = settings.chartBucket;
        if (settings.maxMessages !== undefined) document.getElementById('max-messages').value = settings.maxMessages;
        if (settings.updateInterval !== undefined) document.getElementById('update-interval').value = settings.updateInterval;
    } catch (e) {
        console.error('Failed to load filter settings:', e);
    }
}

function toggleConnection() {
    // Find the connect button
    const btn = document.querySelector('.controls-bar button');
    if (!btn) return;
    
    if (client.isConnected()) {
        client.disconnect();
        btn.textContent = 'Connect';
        stopUpdates();
    } else {
        // Construct WebSocket URL based on current location (same port as HTTP)
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host; // includes port if present
        const wsUrl = `${protocol}//${host}/ws`;
        
        client.onStatus((status, text) => {
            updateConnectionStatus(status, text);
        });

        client.onMessage((message) => {
            messageBuffer.push(message);
        });

        client.onStats((stats) => {
            updateStatsDisplay(stats);
        });

        client.onError((error) => {
            console.error('WebSocket Error:', error);
        });

        client.onConnect(() => {
            // After connection, resend last subscription if available (for reconnection)
            // or apply filters for initial connection
            setTimeout(() => {
                if (client.isConnected()) {
                    if (!client.resendLastSubscription()) {
                        // No previous subscription, apply current filter settings
                        applyFilters();
                    }
                    startUpdates();
                }
            }, 100);
        });

        client.connect(wsUrl);
        btn.textContent = 'Disconnect';
    }
}

function startUpdates() {
    stopUpdates();
    
    loadSettings();
    
    console.log(`Starting updates with interval: ${updateInterval}ms`);
    
    updateTimer = setInterval(() => {
        processMessageBuffer();
        updateChart();
        updateTable();
    }, updateInterval);
    
    // Request stats every 2 seconds
    statsTimer = setInterval(() => {
        client.requestStats();
    }, 2000);
}

function stopUpdates() {
    if (updateTimer) {
        clearInterval(updateTimer);
        updateTimer = null;
    }
    if (statsTimer) {
        clearInterval(statsTimer);
        statsTimer = null;
    }
}

function processMessageBuffer() {
    if (messageBuffer.length === 0) return;
    
    console.log(`Processing ${messageBuffer.length} messages from buffer`);
    
    const now = Date.now();
    const bucketMs = chartBucketSize * 1000;
    
    messageBuffer.forEach(msg => {
        // Add to messages array
        messages.push(msg);
        
        // Add to chart data
        const timestamp = new Date(msg.timestamp).getTime();
        const bucketTime = Math.floor(timestamp / bucketMs) * bucketMs;
        
        if (!chartData.has(bucketTime)) {
            chartData.set(bucketTime, { DEBUG: 0, INFO: 0, WARN: 0, ERROR: 0, FATAL: 0 });
        }
        
        const bucket = chartData.get(bucketTime);
        bucket[msg.level] = (bucket[msg.level] || 0) + 1;
    });
    
    
    messageBuffer = [];
    
    // Trim messages array to max size
    if (messages.length > maxMessages) {
        messages = messages.slice(messages.length - maxMessages);
    }
    
    // Clean old chart data
    const cutoff = now - (chartWindow * 1000);
    for (const [time, _] of chartData.entries()) {
        if (time < cutoff) {
            chartData.delete(time);
        }
    }
}

function updateChart() {
    if (!chart) return;
    
    const sortedTimes = Array.from(chartData.keys()).sort((a, b) => a - b);
    const timestamps = sortedTimes.map(t => new Date(t).toISOString());
    
    const debugData = sortedTimes.map(t => chartData.get(t).DEBUG || 0);
    const infoData = sortedTimes.map(t => chartData.get(t).INFO || 0);
    const warnData = sortedTimes.map(t => chartData.get(t).WARN || 0);
    const errorData = sortedTimes.map(t => chartData.get(t).ERROR || 0);
    const fatalData = sortedTimes.map(t => chartData.get(t).FATAL || 0);
    
    chart.setOption({
        xAxis: {
            data: timestamps
        },
        series: [
            { data: debugData },
            { data: infoData },
            { data: warnData },
            { data: errorData },
            { data: fatalData }
        ]
    }, false); // Use notMerge: false for incremental updates
}

function updateTable() {
    const tbody = document.getElementById('messages-tbody');
    const autoScroll = document.getElementById('auto-scroll').checked;
    const scrollWrapper = tbody.parentElement;
    const wasAtBottom = scrollWrapper.scrollHeight - scrollWrapper.scrollTop - scrollWrapper.clientHeight < 50;
    
    tbody.innerHTML = messages.slice(-1000).reverse().map((msg, idx) => {
        const time = new Date(msg.timestamp);
        const timeStr = time.toLocaleTimeString() + '.' + time.getMilliseconds().toString().padStart(3, '0');
        const hasStackTrace = msg.stack_trace ? '📋' : '';
        
        return `
            <tr onclick="showMessageDetails(${messages.length - 1 - idx})">
                <td>${timeStr}</td>
                <td>${escapeHtml(msg.host)}</td>
                <td title="${escapeHtml(msg.logger)}">${truncate(msg.logger, 25)}</td>
                <td><span class="level-badge level-${msg.level}">${msg.level}</span></td>
                <td class="message-text" title="${escapeHtml(msg.message)}">${escapeHtml(msg.message)}</td>
                <td class="stack-trace-icon">${hasStackTrace}</td>
            </tr>
        `;
    }).join('');
    
    document.getElementById('message-count').textContent = messages.length;
    
    if (autoScroll && wasAtBottom) {
        scrollWrapper.scrollTop = scrollWrapper.scrollHeight;
    }
}

function updateConnectionStatus(status, text) {
    const dot = document.getElementById('status-dot');
    const statusText = document.getElementById('status-text');
    
    dot.className = 'status-dot ' + status;
    statusText.textContent = text;
}

function updateStatsDisplay(stats) {
    const statsInfo = document.getElementById('stats-info');
    // Stats fields from backend: connected, total_clients, queued, dropped
    // - connected: number of currently connected WebSocket clients
    // - total_clients: maximum allowed clients (20)
    // - queued: messages waiting in this client's send buffer
    // - dropped: messages dropped due to rate limiting on this client
    statsInfo.textContent = `Messages: ${messages.length} | Clients: ${stats.connected}/${stats.total_clients} | Dropped: ${stats.dropped}`;
}

function applyFilters() {
    loadSettings();
    
    const levels = [];
    ['debug', 'info', 'warn', 'error', 'fatal'].forEach(level => {
        if (document.getElementById(`level-${level}`).checked) {
            levels.push(level.toUpperCase());
        }
    });
    
    const hostPatterns = document.getElementById('host-patterns').value
        .split('\n')
        .map(s => s.trim())
        .filter(s => s.length > 0);
    
    const loggerPatterns = document.getElementById('logger-patterns').value
        .split('\n')
        .map(s => s.trim())
        .filter(s => s.length > 0);
    
    const messageContains = document.getElementById('message-contains').value
        .split(',')
        .map(s => s.trim())
        .filter(s => s.length > 0);
    
    const messageExcludes = document.getElementById('message-excludes').value
        .split(',')
        .map(s => s.trim())
        .filter(s => s.length > 0);
    
    const messageRegex = document.getElementById('message-regex').value.trim();
    
    const stackTraceMode = document.getElementById('stack-trace-mode').value;
    
    const stackInclude = document.getElementById('stack-include').value
        .split('\n')
        .map(s => s.trim())
        .filter(s => s.length > 0);
    
    const stackExclude = document.getElementById('stack-exclude').value
        .split('\n')
        .map(s => s.trim())
        .filter(s => s.length > 0);
    
    const rateLimit = parseInt(document.getElementById('rate-limit').value);
    const batchTimeout = parseInt(document.getElementById('batch-timeout').value);
    
    const subscription = {
        levels: levels,
        host_patterns: hostPatterns.length > 0 ? hostPatterns : undefined,
        logger_patterns: loggerPatterns.length > 0 ? loggerPatterns : undefined,
        message_contains: messageContains.length > 0 ? messageContains : undefined,
        message_excludes: messageExcludes.length > 0 ? messageExcludes : undefined,
        message_regex: messageRegex || undefined,
        stack_trace_mode: stackTraceMode,
        stack_trace_include: stackInclude.length > 0 ? stackInclude : undefined,
        stack_trace_exclude: stackExclude.length > 0 ? stackExclude : undefined,
        max_rate: rateLimit,
        batch_timeout_ms: batchTimeout
    };
    
    // Remove undefined fields
    Object.keys(subscription).forEach(key => 
        subscription[key] === undefined && delete subscription[key]
    );
    
    console.log('Sending subscription:', subscription);
    
    if (client.isConnected()) {
        client.updateSubscription(subscription);
        // Note: We no longer clear messages - filters apply to new messages only
        // Old messages remain visible until they age out
    }
    
    // Save filter settings to localStorage
    saveFilterSettings();
}

function resetFilters() {
    // Reset to defaults
    document.getElementById('level-debug').checked = false;
    document.getElementById('level-info').checked = true;
    document.getElementById('level-warn').checked = true;
    document.getElementById('level-error').checked = true;
    document.getElementById('level-fatal').checked = true;
    
    document.getElementById('host-patterns').value = '*';
    document.getElementById('logger-patterns').value = '*';
    document.getElementById('message-contains').value = '';
    document.getElementById('message-excludes').value = '';
    document.getElementById('message-regex').value = '';
    document.getElementById('stack-trace-mode').value = 'summary';
    document.getElementById('stack-include').value = '';
    document.getElementById('stack-exclude').value = '';
    document.getElementById('rate-limit').value = '0';
    document.getElementById('batch-timeout').value = '0';
    document.getElementById('chart-window').value = '300';
    document.getElementById('chart-bucket').value = '20';
    document.getElementById('max-messages').value = '10000';
    document.getElementById('update-interval').value = '250';
    
    // Clear saved settings
    localStorage.removeItem('streamFilterSettings');
    
    applyFilters();
}

function clearMessages() {
    messages = [];
    messageBuffer = [];
    chartData.clear();
    updateChart();
    updateTable();
}

function showMessageDetails(index) {
    const msg = messages[index];
    if (!msg) return;
    
    const modal = document.getElementById('detail-modal');
    const title = document.getElementById('modal-title');
    const body = document.getElementById('modal-body');
    
    let content = `Timestamp: ${msg.timestamp}\n`;
    content += `Host: ${msg.host}\n`;
    content += `Logger: ${msg.logger}\n`;
    content += `Level: ${msg.level}\n\n`;
    content += `Message:\n${msg.message}\n`;
    
    if (msg.stack_trace) {
        content += `\n${'='.repeat(60)}\nStack Trace:\n${'='.repeat(60)}\n`;
        
        if (msg.stack_trace.hash) {
            // Summary or filtered mode
            content += `Hash: ${msg.stack_trace.hash}\n\n`;
            
            if (msg.stack_trace.first_line) {
                // Summary mode
                content += `First Line: ${msg.stack_trace.first_line}\n`;
                content += `Total Frames: ${msg.stack_trace.frame_count}\n`;
            } else if (msg.stack_trace.frames) {
                // Filtered mode
                content += `Relevant Frames:\n`;
                msg.stack_trace.frames.forEach((frame, i) => {
                    content += `  ${i + 1}. ${frame}\n`;
                });
                content += `\nOmitted Frames: ${msg.stack_trace.omitted}\n`;
            }
        }
    }
    
    title.textContent = `Message Details - ${msg.level}`;
    body.textContent = content;
    modal.style.display = 'block';
}

function closeModal() {
    document.getElementById('detail-modal').style.display = 'none';
}

function copyModalContent() {
    const content = document.getElementById('modal-body').textContent;
    navigator.clipboard.writeText(content).then(() => {
        alert('Copied to clipboard!');
    }).catch(err => {
        console.error('Failed to copy:', err);
    });
}

// Close modal when clicking outside
window.onclick = (event) => {
    const modal = document.getElementById('detail-modal');
    if (event.target === modal) {
        closeModal();
    }
};

// Utility functions
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function truncate(text, maxLength) {
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength - 3) + '...';
}

// Page visibility API - pause updates when tab not visible
document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        stopUpdates();
    } else if (client.isConnected()) {
        startUpdates();
    }
});
