package api

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/muozez/ephem-centralized-access-broker/internal/auth"
	"github.com/muozez/ephem-centralized-access-broker/internal/db"
)

var loginTemplate = template.Must(template.New("login").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ephem Auth Broker - Sign In</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #030712;
            --card-bg: rgba(17, 24, 39, 0.7);
            --border: rgba(255, 255, 255, 0.08);
            --primary: #10b981;
            --primary-glow: rgba(16, 185, 129, 0.15);
            --text: #f3f4f6;
            --text-muted: #9ca3af;
        }
        body {
            background-color: var(--bg);
            color: var(--text);
            font-family: 'Outfit', sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            overflow: hidden;
            position: relative;
        }
        body::before, body::after {
            content: '';
            position: absolute;
            width: 400px;
            height: 400px;
            border-radius: 50%;
            background: radial-gradient(circle, var(--primary-glow) 0%, rgba(0,0,0,0) 70%);
            z-index: -1;
        }
        body::before {
            top: -100px;
            left: -100px;
        }
        body::after {
            bottom: -100px;
            right: -100px;
        }
        .container {
            background: var(--card-bg);
            backdrop-filter: blur(20px);
            -webkit-backdrop-filter: blur(20px);
            border: 1px solid var(--border);
            border-radius: 24px;
            padding: 48px;
            width: 100%;
            max-width: 400px;
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
            box-sizing: border-box;
            transform: translateY(0);
            transition: all 0.3s ease;
        }
        .container:hover {
            border-color: rgba(16, 185, 129, 0.3);
            box-shadow: 0 25px 50px -12px rgba(16, 185, 129, 0.05);
        }
        .logo {
            font-size: 2.2rem;
            font-weight: 700;
            color: var(--primary);
            margin-bottom: 8px;
            text-align: center;
            letter-spacing: -0.05em;
        }
        .subtitle {
            font-size: 0.95rem;
            color: var(--text-muted);
            margin-bottom: 32px;
            text-align: center;
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            font-size: 0.85rem;
            font-weight: 600;
            margin-bottom: 8px;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        input, select {
            width: 100%;
            padding: 14px;
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid var(--border);
            border-radius: 12px;
            color: var(--text);
            font-family: inherit;
            font-size: 1rem;
            box-sizing: border-box;
            transition: all 0.2s ease;
        }
        input:focus, select:focus {
            outline: none;
            border-color: var(--primary);
            background: rgba(255, 255, 255, 0.06);
            box-shadow: 0 0 0 4px var(--primary-glow);
        }
        .btn {
            width: 100%;
            padding: 16px;
            background: var(--primary);
            border: none;
            border-radius: 12px;
            color: #030712;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s ease;
            margin-top: 10px;
        }
        .btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 20px -10px var(--primary);
            background: #34d399;
        }
        .btn:active {
            transform: translateY(0);
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">ephem</div>
        <div class="subtitle">Infrastructure access, issued on demand.</div>
        
        <form method="POST" action="/v1/auth/callback">
            <input type="hidden" name="cli_port" value="{{.CliPort}}">
            
            <div class="form-group">
                <label for="email">Email Address</label>
                <input type="email" id="email" name="email" required placeholder="name@company.com" value="developer@company.com">
            </div>
            
            <div class="form-group">
                <label for="name">Full Name</label>
                <input type="text" id="name" name="name" required placeholder="John Doe" value="John Doe">
            </div>
            
            <div class="form-group">
                <label for="role">Select Role</label>
                <select id="role" name="role">
                    <option value="developer">Developer</option>
                    <option value="admin">Administrator</option>
                    <option value="dba">Database Administrator (DBA)</option>
                    <option value="devops">DevOps Engineer</option>
                </select>
            </div>
            
            <button type="submit" class="btn">Authenticate</button>
        </form>
    </div>
</body>
</html>
`))

type loginPageData struct {
	CliPort string
}

// HandleLogin renders the mock OIDC login page
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cliPort := r.URL.Query().Get("cli_port")

	data := loginPageData{
		CliPort: cliPort,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loginTemplate.Execute(w, data)
}

// HandleCallback processes OIDC authentication callback, saves user details to database,
// and redirects back to CLI callback server.
func HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	name := r.FormValue("name")
	role := r.FormValue("role")
	cliPort := r.FormValue("cli_port")

	if email == "" || name == "" || role == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// 1. Save or Update User in the Database
	conn, err := db.Connect()
	if err != nil {
		http.Error(w, fmt.Sprintf("Database connection failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	var userID string
	err = conn.QueryRow(`
		INSERT INTO users (email, name, provider, external_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE SET name = $2, external_id = $4
		RETURNING id
	`, email, name, "oidc", "ext-"+email).Scan(&userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save user: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Ensure Role exists and Link User
	var roleID string
	err = conn.QueryRow(`
		INSERT INTO roles (name)
		VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, role).Scan(&roleID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create role: %v", err), http.StatusInternalServerError)
		return
	}

	_, err = conn.Exec(`
		INSERT INTO user_roles (user_id, role_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, role_id) DO NOTHING
	`, userID, roleID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to associate user and role: %v", err), http.StatusInternalServerError)
		return
	}

	// 3. Generate JWT Token
	token, err := auth.GenerateToken(userID, email, name, []string{role})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate JWT: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Log the login event in audit logs
	_, _ = conn.Exec(`
		INSERT INTO audit_logs (user_id, action, result, created_at)
		VALUES ($1, 'login', 'SUCCESS', NOW())
	`, userID)

	// 5. Redirect back to CLI port callback URL if available
	if cliPort != "" {
		redirectURL := fmt.Sprintf("http://127.0.0.1:%s/callback?token=%s", cliPort, token)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Default display page if CLI port wasn't provided
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>ephem - Auth Success</title>
			<style>
				body { font-family: sans-serif; background-color: #0b0f19; color: #f3f4f6; padding: 50px; text-align: center; }
				.token { background: #1e293b; padding: 20px; border-radius: 8px; font-family: monospace; word-break: break-all; margin: 20px auto; max-width: 600px; text-align: left; }
			</style>
		</head>
		<body>
			<h1>Authenticated Successfully</h1>
			<p>Here is your CLI token:</p>
			<div class="token">%s</div>
		</body>
		</html>
	`, token)))
}
