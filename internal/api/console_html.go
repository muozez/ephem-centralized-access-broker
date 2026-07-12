package api

const AdminConsoleHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ephem — Centralized Access Management Console</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=Outfit:wght@400;500;600;700;800&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #090d16;
            --card-bg: rgba(22, 31, 48, 0.7);
            --card-border: rgba(255, 255, 255, 0.08);
            --text-main: #f3f4f6;
            --text-muted: #9ca3af;
            --primary: #3b82f6;
            --primary-gradient: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
            --accent: #8b5cf6;
            --success: #10b981;
            --warning: #f59e0b;
            --danger: #ef4444;
            --sidebar-width: 260px;
        }

        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            font-family: 'Inter', sans-serif;
            background-color: var(--bg);
            color: var(--text-main);
            min-height: 100vh;
            display: flex;
            overflow-x: hidden;
        }

        /* Scrollbar styling */
        ::-webkit-scrollbar {
            width: 8px;
            height: 8px;
        }
        ::-webkit-scrollbar-track {
            background: var(--bg);
        }
        ::-webkit-scrollbar-thumb {
            background: rgba(255, 255, 255, 0.1);
            border-radius: 4px;
        }
        ::-webkit-scrollbar-thumb:hover {
            background: rgba(255, 255, 255, 0.2);
        }

        /* Sidebar styling */
        .sidebar {
            width: var(--sidebar-width);
            background: rgba(11, 15, 25, 0.95);
            border-right: 1px solid var(--card-border);
            display: flex;
            flex-direction: column;
            padding: 2rem 1.5rem;
            position: fixed;
            height: 100vh;
            z-index: 10;
        }

        .logo-container {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            margin-bottom: 3rem;
        }

        .logo-icon {
            width: 32px;
            height: 32px;
            background: var(--primary-gradient);
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 800;
            font-family: 'Outfit', sans-serif;
            color: white;
            box-shadow: 0 0 15px rgba(59, 130, 246, 0.4);
        }

        .logo-text {
            font-family: 'Outfit', sans-serif;
            font-size: 1.5rem;
            font-weight: 800;
            letter-spacing: -0.5px;
            background: var(--primary-gradient);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .nav-links {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
            list-style: none;
        }

        .nav-item {
            display: flex;
            align-items: center;
            gap: 1rem;
            padding: 0.75rem 1rem;
            color: var(--text-muted);
            text-decoration: none;
            border-radius: 8px;
            font-weight: 500;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            cursor: pointer;
        }

        .nav-item:hover, .nav-item.active {
            color: var(--text-main);
            background: rgba(255, 255, 255, 0.05);
        }

        .nav-item.active {
            border-left: 3px solid var(--primary);
            background: rgba(59, 130, 246, 0.1);
        }

        /* Main Content area */
        .main-content {
            margin-left: var(--sidebar-width);
            flex-grow: 1;
            padding: 2.5rem;
            min-width: 0;
        }

        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 3rem;
        }

        .page-title {
            font-family: 'Outfit', sans-serif;
            font-size: 2rem;
            font-weight: 700;
            letter-spacing: -0.5px;
        }

        .admin-profile {
            display: flex;
            align-items: center;
            gap: 1rem;
            background: var(--card-bg);
            border: 1px solid var(--card-border);
            padding: 0.5rem 1rem;
            border-radius: 20px;
            backdrop-filter: blur(10px);
        }

        .admin-avatar {
            width: 24px;
            height: 24px;
            background: var(--accent);
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 0.75rem;
            font-weight: 700;
        }

        .btn {
            background: var(--primary-gradient);
            color: white;
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s ease;
        }

        .btn:hover {
            opacity: 0.9;
            transform: translateY(-1px);
        }

        .btn-danger {
            background: var(--danger);
        }

        .btn-outline {
            background: transparent;
            border: 1px solid var(--card-border);
            color: var(--text-muted);
        }

        .btn-outline:hover {
            border-color: var(--text-main);
            color: var(--text-main);
        }

        /* Grid metrics cards */
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
            gap: 1.5rem;
            margin-bottom: 3rem;
        }

        .metric-card {
            background: var(--card-bg);
            border: 1px solid var(--card-border);
            border-radius: 12px;
            padding: 1.5rem;
            backdrop-filter: blur(12px);
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }

        .metric-title {
            font-size: 0.875rem;
            color: var(--text-muted);
            font-weight: 500;
        }

        .metric-value {
            font-size: 2rem;
            font-family: 'Outfit', sans-serif;
            font-weight: 700;
            background: var(--primary-gradient);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        /* Card panels */
        .panel {
            background: var(--card-bg);
            border: 1px solid var(--card-border);
            border-radius: 16px;
            padding: 2rem;
            backdrop-filter: blur(15px);
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
            margin-bottom: 2rem;
            display: none;
            animation: fadeIn 0.4s ease forwards;
        }

        .panel.active {
            display: block;
        }

        .panel-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
            border-bottom: 1px solid var(--card-border);
            padding-bottom: 1rem;
        }

        .panel-title {
            font-family: 'Outfit', sans-serif;
            font-size: 1.25rem;
            font-weight: 600;
        }

        /* Table styles */
        table {
            width: 100%;
            border-collapse: collapse;
            text-align: left;
        }

        th {
            padding: 1rem;
            font-size: 0.75rem;
            text-transform: uppercase;
            color: var(--text-muted);
            font-weight: 600;
            border-bottom: 1px solid var(--card-border);
        }

        td {
            padding: 1rem;
            border-bottom: 1px solid var(--card-border);
            font-size: 0.875rem;
        }

        tr:last-child td {
            border-bottom: none;
        }

        .badge {
            display: inline-block;
            padding: 0.25rem 0.5rem;
            border-radius: 12px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
        }

        .badge-active {
            background: rgba(16, 185, 129, 0.15);
            color: var(--success);
        }

        .badge-revoked, .badge-expired {
            background: rgba(239, 68, 68, 0.15);
            color: var(--danger);
        }

        .badge-success {
            background: rgba(16, 185, 129, 0.15);
            color: var(--success);
        }

        .badge-error, .badge-denied {
            background: rgba(239, 68, 68, 0.15);
            color: var(--danger);
        }

        /* Auth Screen overlay */
        .auth-overlay {
            position: fixed;
            top: 0;
            left: 0;
            width: 100vw;
            height: 100vh;
            background: rgba(5, 7, 12, 0.95);
            z-index: 100;
            display: flex;
            align-items: center;
            justify-content: center;
            backdrop-filter: blur(20px);
        }

        .auth-card {
            background: rgba(22, 31, 48, 0.8);
            border: 1px solid var(--card-border);
            border-radius: 20px;
            width: 480px;
            padding: 3rem;
            display: flex;
            flex-direction: column;
            gap: 1.5rem;
            box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
            animation: scaleIn 0.3s cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
        }

        .auth-title {
            font-family: 'Outfit', sans-serif;
            font-size: 1.75rem;
            font-weight: 700;
            text-align: center;
        }

        .auth-subtitle {
            font-size: 0.875rem;
            color: var(--text-muted);
            text-align: center;
            line-height: 1.5;
        }

        .form-group {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }

        .form-label {
            font-size: 0.75rem;
            font-weight: 600;
            color: var(--text-muted);
            text-transform: uppercase;
        }

        .form-input {
            background: rgba(11, 15, 25, 0.8);
            border: 1px solid var(--card-border);
            padding: 0.75rem 1rem;
            border-radius: 8px;
            color: white;
            font-family: inherit;
            font-size: 0.875rem;
            transition: border-color 0.2s ease;
        }

        .form-input:focus {
            outline: none;
            border-color: var(--primary);
        }

        .error-banner {
            background: rgba(239, 68, 68, 0.15);
            border: 1px solid var(--danger);
            color: var(--danger);
            padding: 0.75rem 1rem;
            border-radius: 8px;
            font-size: 0.875rem;
            display: none;
        }

        /* Animations */
        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(10px); }
            to { opacity: 1; transform: translateY(0); }
        }

        @keyframes scaleIn {
            from { opacity: 0; transform: scale(0.95); }
            to { opacity: 1; transform: scale(1); }
        }

        /* Grid Layouts for lists */
        .cards-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
            gap: 1.5rem;
        }

        .grid-card {
            background: rgba(255, 255, 255, 0.02);
            border: 1px solid var(--card-border);
            border-radius: 12px;
            padding: 1.5rem;
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }

        .grid-card-title {
            font-family: 'Outfit', sans-serif;
            font-weight: 600;
            font-size: 1.125rem;
        }

        .grid-card-item {
            display: flex;
            justify-content: space-between;
            font-size: 0.875rem;
        }

        .grid-card-label {
            color: var(--text-muted);
        }
    </style>
</head>
<body>

    <!-- Sidebar navigation -->
    <div class="sidebar">
        <div class="logo-container">
            <div class="logo-icon">e</div>
            <div class="logo-text">ephem</div>
        </div>
        <ul class="nav-links">
            <li><a class="nav-item active" onclick="switchTab('dashboard')">Dashboard</a></li>
            <li><a class="nav-item" onclick="switchTab('sessions')">Sessions</a></li>
            <li><a class="nav-item" onclick="switchTab('resources')">Resources</a></li>
            <li><a class="nav-item" onclick="switchTab('policies')">Policies</a></li>
            <li><a class="nav-item" onclick="switchTab('audit')">Audit History</a></li>
        </ul>
    </div>

    <!-- Main Container -->
    <div class="main-content">
        <header>
            <div class="page-title" id="tab-title">Dashboard</div>
            <div class="admin-profile">
                <div class="admin-avatar" id="avatar-char">A</div>
                <span id="admin-name">Admin User</span>
                <button class="btn btn-outline" onclick="logout()">Logout</button>
            </div>
        </header>

        <!-- Metric summaries (Dashboard Home only) -->
        <div id="metrics-bar" class="metrics-grid">
            <div class="metric-card">
                <span class="metric-title">Active Sessions</span>
                <span class="metric-value" id="count-active">0</span>
            </div>
            <div class="metric-card">
                <span class="metric-title">Total Resources</span>
                <span class="metric-value" id="count-resources">0</span>
            </div>
            <div class="metric-card">
                <span class="metric-title">Active Policies</span>
                <span class="metric-value" id="count-policies">0</span>
            </div>
            <div class="metric-card">
                <span class="metric-title">Failed Audits</span>
                <span class="metric-value" id="count-failed-audits">0</span>
            </div>
        </div>

        <!-- 1. Dashboard View -->
        <div id="panel-dashboard" class="panel active">
            <div class="panel-header">
                <h3 class="panel-title">System Status Overview</h3>
            </div>
            <p style="color: var(--text-muted); line-height: 1.6; margin-bottom: 1rem;">
                Welcome to the ephem Centralized Access Control Console. This dashboard monitors temporary database roles and signed SSH certificate sessions.
            </p>
            <p style="color: var(--text-muted); line-height: 1.6;">
                Use the sidebar navigation to audit access logs, revoke rogue sessions, and check policy configurations.
            </p>
        </div>

        <!-- 2. Sessions View -->
        <div id="panel-sessions" class="panel">
            <div class="panel-header">
                <h3 class="panel-title">Active & Expired Sessions</h3>
            </div>
            <table id="sessions-table">
                <thead>
                    <tr>
                        <th>Session ID</th>
                        <th>User</th>
                        <th>Resource</th>
                        <th>Provider</th>
                        <th>Issued At</th>
                        <th>Expires At</th>
                        <th>Status</th>
                        <th>Action</th>
                    </tr>
                </thead>
                <tbody id="sessions-tbody">
                    <!-- Session rows inserted dynamically -->
                </tbody>
            </table>
        </div>

        <!-- 3. Resources View -->
        <div id="panel-resources" class="panel">
            <div class="panel-header">
                <h3 class="panel-title">Registered Targets</h3>
            </div>
            <div class="cards-grid" id="resources-grid">
                <!-- Resources grid cards -->
            </div>
        </div>

        <!-- 4. Policies View -->
        <div id="panel-policies" class="panel">
            <div class="panel-header">
                <h3 class="panel-title">Access Policies</h3>
            </div>
            <table id="policies-table">
                <thead>
                    <tr>
                        <th>Role</th>
                        <th>Resource Pattern</th>
                        <th>Effect</th>
                        <th>Max Duration</th>
                    </tr>
                </thead>
                <tbody id="policies-tbody">
                    <!-- Policies inserted dynamically -->
                </tbody>
            </table>
        </div>

        <!-- 5. Audit Log View -->
        <div id="panel-audit" class="panel">
            <div class="panel-header">
                <h3 class="panel-title">Security Audit Log (Write-Only History)</h3>
            </div>
            <table id="audit-table">
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>User</th>
                        <th>Action</th>
                        <th>Target Resource</th>
                        <th>Result</th>
                        <th>Details/Reason</th>
                        <th>Time</th>
                    </tr>
                </thead>
                <tbody id="audit-tbody">
                    <!-- Audits inserted dynamically -->
                </tbody>
            </table>
        </div>
    </div>

    <!-- Login Modal -->
    <div class="auth-overlay" id="auth-overlay">
        <div class="auth-card">
            <div class="auth-title">Authentication Required</div>
            <div class="auth-subtitle">
                Access is restricted to authorized administrator roles. Please paste the JWT admin token from your ephem local configurations.
            </div>
            <div class="error-banner" id="auth-error">Invalid Token or Forbidden Role</div>
            <div class="form-group">
                <label class="form-label">Admin JWT Token</label>
                <textarea class="form-input" id="auth-token" rows="6" placeholder="Bearer Token..."></textarea>
            </div>
            <button class="btn" onclick="authenticate()">Sign In</button>
        </div>
    </div>

    <script>
        // State variables
        var currentTab = 'dashboard';
        var token = localStorage.getItem('ephem_admin_token') || '';

        // Startup verification
        window.addEventListener('DOMContentLoaded', function() {
            if (!token) {
                showAuthScreen();
            } else {
                hideAuthScreen();
                loadAllData();
            }
        });

        function showAuthScreen() {
            document.getElementById('auth-overlay').style.display = 'flex';
        }

        function hideAuthScreen() {
            document.getElementById('auth-overlay').style.display = 'none';
        }

        function logout() {
            localStorage.removeItem('ephem_admin_token');
            token = '';
            showAuthScreen();
        }

        function authenticate() {
            var rawToken = document.getElementById('auth-token').value.trim();
            if (!rawToken) return;

            // Remove Bearer prefix if added
            var cleanedToken = rawToken.replace(/^Bearer\s+/i, '');
            token = cleanedToken;
            localStorage.setItem('ephem_admin_token', token);

            // Test authentication by loading data
            loadAllData().then(function(success) {
                if (success) {
                    hideAuthScreen();
                    document.getElementById('auth-error').style.display = 'none';
                } else {
                    document.getElementById('auth-error').style.display = 'block';
                    logout();
                }
            });
        }

        // Helper to format ISO time to locale string
        function formatTime(isoStr) {
            if (!isoStr) return '-';
            var date = new Date(isoStr);
            return date.toLocaleString();
        }

        // Parse JWT payload to extract user name
        function parseJWT(token) {
            try {
                var base64Url = token.split('.')[1];
                var base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
                return JSON.parse(window.atob(base64));
            } catch (e) {
                return null;
            }
        }

        async function fetchAPI(endpoint, method, body) {
            if (!method) method = 'GET';
            var headers = {
                'Authorization': 'Bearer ' + token
            };
            if (body) {
                headers['Content-Type'] = 'application/json';
            }

            try {
                var res = await fetch(endpoint, {
                    method: method,
                    headers: headers,
                    body: body ? JSON.stringify(body) : null
                });
                if (res.status === 401 || res.status === 403) {
                    logout();
                    return null;
                }
                if (!res.ok) throw new Error(await res.text());
                return await res.json();
            } catch (e) {
                console.error('API Error on ' + endpoint + ':', e);
                return null;
            }
        }

        async function loadAllData() {
            // Extract profile info from JWT
            var claims = parseJWT(token);
            if (!claims) return false;

            document.getElementById('admin-name').innerText = claims.name || claims.email;
            document.getElementById('avatar-char').innerText = (claims.name || 'A')[0].toUpperCase();

            // Load data parallelly
            var sessions = await fetchAPI('/v1/admin/sessions');
            if (!sessions) return false;

            var resources = await fetchAPI('/v1/admin/resources');
            var policies = await fetchAPI('/v1/admin/policies');
            var audits = await fetchAPI('/v1/admin/audit_logs');

            renderSessions(sessions);
            renderResources(resources);
            renderPolicies(policies);
            renderAudits(audits);
            updateDashboardMetrics(sessions, resources, policies, audits);

            return true;
        }

        function updateDashboardMetrics(sessions, resources, policies, audits) {
            if (sessions) {
                var activeCount = sessions.filter(function(s) { return s.status === 'ACTIVE'; }).length;
                document.getElementById('count-active').innerText = activeCount;
            }
            if (resources) {
                document.getElementById('count-resources').innerText = resources.length;
            }
            if (policies) {
                document.getElementById('count-policies').innerText = policies.length;
            }
            if (audits) {
                var failed = audits.filter(function(a) { return a.result === 'ERROR' || a.result === 'DENIED'; }).length;
                document.getElementById('count-failed-audits').innerText = failed;
            }
        }

        function renderSessions(sessions) {
            var tbody = document.getElementById('sessions-tbody');
            tbody.innerHTML = '';
            if (!sessions || sessions.length === 0) {
                tbody.innerHTML = '<tr><td colspan="8" style="text-align: center; color: var(--text-muted);">No sessions recorded.</td></tr>';
                return;
            }

            sessions.forEach(function(s) {
                var tr = document.createElement('tr');
                var badgeClass = 'badge badge-' + s.status.toLowerCase();
                var revokeBtn = s.status === 'ACTIVE' 
                    ? '<button class="btn btn-danger" onclick="revokeSession(\'' + s.id + '\')" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;">Revoke</button>'
                    : '-';

                tr.innerHTML = '<tr>' +
                    '<td style="font-family: monospace;">' + s.id.substring(0, 8) + '...</td>' +
                    '<td>' + s.user_id + '</td>' +
                    '<td>' + s.resource_name + '</td>' +
                    '<td><span class="badge" style="background: rgba(255,255,255,0.05);">' + s.provider + '</span></td>' +
                    '<td>' + formatTime(s.issued_at) + '</td>' +
                    '<td>' + formatTime(s.expires_at) + '</td>' +
                    '<td><span class="' + badgeClass + '">' + s.status + '</span></td>' +
                    '<td>' + revokeBtn + '</td>' +
                '</tr>';
                tbody.appendChild(tr);
            });
        }

        function renderResources(resources) {
            var grid = document.getElementById('resources-grid');
            grid.innerHTML = '';
            if (!resources || resources.length === 0) {
                grid.innerHTML = '<p style="color: var(--text-muted);">No resources configured.</p>';
                return;
            }

            resources.forEach(function(r) {
                var card = document.createElement('div');
                card.className = 'grid-card';
                card.innerHTML = 
                    '<div class="grid-card-title">' + (r.display_name || r.name) + '</div>' +
                    '<div style="display: flex; flex-direction: column; gap: 0.5rem;">' +
                        '<div class="grid-card-item">' +
                            '<span class="grid-card-label">Identifier</span>' +
                            '<span>' + r.name + '</span>' +
                        '</div>' +
                        '<div class="grid-card-item">' +
                            '<span class="grid-card-label">Provider</span>' +
                            '<span>' + r.provider + '</span>' +
                        '</div>' +
                        '<div class="grid-card-item">' +
                            '<span class="grid-card-label">Env</span>' +
                            '<span>' + r.environment + '</span>' +
                        '</div>' +
                        '<div class="grid-card-item">' +
                            '<span class="grid-card-label">Team</span>' +
                            '<span>' + (r.owner_team || '-') + '</span>' +
                        '</div>' +
                        '<div class="grid-card-item">' +
                            '<span class="grid-card-label">Status</span>' +
                            '<span class="badge badge-' + (r.enabled ? 'active' : 'revoked') + '">' + (r.enabled ? 'Enabled' : 'Disabled') + '</span>' +
                        '</div>' +
                    '</div>';
                grid.appendChild(card);
            });
        }

        function renderPolicies(policies) {
            var tbody = document.getElementById('policies-tbody');
            tbody.innerHTML = '';
            if (!policies || policies.length === 0) {
                tbody.innerHTML = '<tr><td colspan="4" style="text-align: center; color: var(--text-muted);">No policies loaded.</td></tr>';
                return;
            }

            policies.forEach(function(p) {
                var tr = document.createElement('tr');
                var maxDur = p.conditions && p.conditions.max_duration ? p.conditions.max_duration : '30m';
                tr.innerHTML = 
                    '<td><strong>' + p.role_name + '</strong></td>' +
                    '<td style="font-family: monospace;">' + p.resource_glob + '</td>' +
                    '<td><span class="badge" style="background: rgba(16, 185, 129, 0.15); color: var(--success);">' + p.effect + '</span></td>' +
                    '<td>' + maxDur + '</td>';
                tbody.appendChild(tr);
            });
        }

        function renderAudits(audits) {
            var tbody = document.getElementById('audit-tbody');
            tbody.innerHTML = '';
            if (!audits || audits.length === 0) {
                tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No audit history found.</td></tr>';
                return;
            }

            audits.forEach(function(a) {
                var tr = document.createElement('tr');
                var resClass = 'badge badge-' + a.result.toLowerCase();
                tr.innerHTML = 
                    '<td>' + a.id + '</td>' +
                    '<td>' + a.user_id + '</td>' +
                    '<td style="font-family: monospace; font-size: 0.8rem;">' + a.action + '</td>' +
                    '<td>' + a.resource_name + '</td>' +
                    '<td><span class="' + resClass + '">' + a.result + '</span></td>' +
                    '<td style="color: var(--text-muted); font-size: 0.8rem;">' + (a.reason || '-') + '</td>' +
                    '<td>' + formatTime(a.created_at) + '</td>';
                tbody.appendChild(tr);
            });
        }

        async function revokeSession(sessionID) {
            if (!confirm('Are you sure you want to revoke session ' + sessionID + '?')) return;

            var res = await fetchAPI('/v1/sessions/revoke', 'POST', { session_id: sessionID });
            if (res) {
                alert('Session revoked successfully.');
                loadAllData();
            } else {
                alert('Failed to revoke session.');
            }
        }

        function switchTab(tabId) {
            document.querySelectorAll('.nav-item').forEach(function(el) { el.classList.remove('active'); });
            document.querySelectorAll('.panel').forEach(function(el) { el.classList.remove('active'); });

            var clickedLink = Array.from(document.querySelectorAll('.nav-item')).find(function(el) {
                return el.getAttribute('onclick').includes(tabId);
            });
            if (clickedLink) clickedLink.classList.add('active');

            var targetPanel = document.getElementById('panel-' + tabId);
            if (targetPanel) targetPanel.classList.add('active');

            var titles = {
                'dashboard': 'Dashboard',
                'sessions': 'Sessions Manager',
                'resources': 'Resources Directory',
                'policies': 'Access Policies',
                'audit': 'Audit History'
            };
            document.getElementById('tab-title').innerText = titles[tabId] || 'Management Console';

            var metricsBar = document.getElementById('metrics-bar');
            if (tabId === 'dashboard') {
                metricsBar.style.display = 'grid';
            } else {
                metricsBar.style.display = 'none';
            }

            currentTab = tabId;
            loadAllData();
        }

        setInterval(function() {
            if (token) {
                loadAllData();
            }
        }, 10000);
    </script>
</body>
</html>
`
