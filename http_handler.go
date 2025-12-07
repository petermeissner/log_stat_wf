package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

func startHTTPServer(addr string, store *LogStatStore) {
	app := fiber.New(fiber.Config{
		AppName: "WildFly Log Statistics",
	})

	// API endpoint for stats
	app.Get("/api/stats", func(c *fiber.Ctx) error {
		stats := store.GetAll()
		return c.JSON(stats)
	})

	// Serve HTML dashboard
	app.Get("/", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(getDashboardHTML())
	})

	// Serve static files
	app.Static("/static", "./static")

	log.Printf("Fiber HTTP server starting on %s\n", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("HTTP server error: %v\n", err)
	}
}

func getDashboardHTML() string {
	return `<!DOCTYPE html>
<html>
<head>
    <title>WildFly Log Statistics</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        h1 { color: #333; }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            padding: 20px;
            border-radius: 5px;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background-color: #4CAF50;
            color: white;
        }
        tr:hover { background-color: #f5f5f5; }
        .refresh-btn {
            padding: 10px 20px;
            background-color: #4CAF50;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }
        .refresh-btn:hover { background-color: #45a049; }
        .loading {
            text-align: center;
            padding: 20px;
            color: #666;
        }
        .error {
            color: #d32f2f;
            padding: 10px;
            background-color: #ffebee;
            border-radius: 4px;
            margin-top: 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>WildFly Log Statistics</h1>
        <button class="refresh-btn" onclick="loadStats()">Refresh</button>
        <div id="error" class="error" style="display: none;"></div>
        <div id="loading" class="loading" style="display: none;">Loading statistics...</div>
        <table id="statsTable" style="display: none;">
            <thead>
                <tr>
                    <th>Host</th>
                    <th>Logger</th>
                    <th>Level</th>
                    <th>Count</th>
                    <th>First Seen</th>
                    <th>Duration (seconds)</th>
                </tr>
            </thead>
            <tbody id="statsBody"></tbody>
        </table>
    </div>
    <script>
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
		loadStats();
		setInterval(loadStats, 5000);
	</script>
</body>
</html>`
}
