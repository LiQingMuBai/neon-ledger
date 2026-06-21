package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"bookkeeping-api/internal/config"
	"bookkeeping-api/internal/notify"
	"bookkeeping-api/internal/orders"
	"bookkeeping-api/internal/web"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if err := config.LoadEnvFile(".env"); err != nil {
		log.Fatal(err)
	}

	addr := getenv("ADDR", ":5555")
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY is required in .env")
	}
	webUsername := os.Getenv("WEB_USERNAME")
	webPassword := os.Getenv("WEB_PASSWORD")
	sessionSecret := os.Getenv("SESSION_SECRET")
	if webUsername == "" || webPassword == "" {
		log.Fatal("WEB_USERNAME and WEB_PASSWORD are required in .env")
	}
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET is required in .env")
	}
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN is required in .env")
	}
	apiTestEnabled := parseBoolEnv("API_TEST_ENABLED", true)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("mysql connected")
	if err := orders.EnsureMySQLSchema(ctx, db); err != nil {
		log.Fatal(err)
	}
	log.Println("mysql schema ensured")

	store := orders.NewMySQLStore(db)
	adminChatID, err := parseOptionalInt64(os.Getenv("TELEGRAM_ADMIN_CHAT_ID"))
	if err != nil {
		log.Fatal(err)
	}
	notifier := notify.NewTelegramNotifier(os.Getenv("TELEGRAM_BOT_TOKEN"), getenv("TELEGRAM_API_BASE", "https://api.telegram.org"), adminChatID)
	handler := orders.NewHandlerWithNotifier(store, notifier)

	mux := http.NewServeMux()
	mux.HandleFunc("/captcha", captchaHandler(sessionSecret))
	mux.HandleFunc("/login", loginHandler(webUsername, webPassword, sessionSecret))
	mux.HandleFunc("/logout", logoutHandler())
	mux.HandleFunc("/app-config", appConfigHandler(apiTestEnabled))
	handler.RegisterRoutes(mux)
	web.RegisterRoutes(mux, apiTestEnabled)

	log.Printf("bookkeeping api listening on %s", addr)
	if err := http.ListenAndServe(addr, logRequests(requireAuth(mux, apiKey, sessionSecret, apiTestEnabled))); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseOptionalInt64(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func parseBoolEnv(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value != "false"
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		log.Printf(
			"access method=%s path=%q status=%d duration=%s remote=%s",
			r.Method,
			r.URL.RequestURI(),
			recorder.status,
			time.Since(startedAt).Round(time.Microsecond),
			clientIP(r),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func clientIP(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}

func requireAuth(next http.Handler, apiKey, sessionSecret string, apiTestEnabled bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiKeyValid := validAPIKey(r, apiKey)
			sessionValid := validSession(r, sessionSecret)
			if apiKeyValid && !sessionValid && isExternalOrderMutation(r.URL.Path, r.Method) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusMethodNotAllowed)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
				return
			}
			if apiKeyValid || sessionValid {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		if isProtectedPage(r.URL.Path, apiTestEnabled) && !validSession(r, sessionSecret) {
			nextURL := r.URL.RequestURI()
			http.Redirect(w, r, "/login?next="+urlQueryEscape(nextURL), http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func appConfigHandler(apiTestEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]bool{"api_test_enabled": apiTestEnabled})
	}
}

func loginHandler(username, password, sessionSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			web.ServeLogin(w, r)
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Redirect(w, r, "/login?error=1", http.StatusFound)
				return
			}
			nextURL := safeNextURL(r.FormValue("next"))
			if validCaptcha(r, sessionSecret, r.FormValue("captcha")) &&
				constantTimeEqual(r.FormValue("username"), username) &&
				constantTimeEqual(r.FormValue("password"), password) {
				http.SetCookie(w, sessionCookie(makeSessionValue(username, sessionSecret, time.Now().Add(12*time.Hour))))
				http.SetCookie(w, expiredCookie("bookkeeping_captcha"))
				http.Redirect(w, r, nextURL, http.StatusFound)
				return
			}
			http.Redirect(w, r, "/login?error=1&next="+urlQueryEscape(nextURL), http.StatusFound)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func logoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "bookkeeping_session",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func captchaHandler(sessionSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		left := randomInt(1, 9)
		right := randomInt(1, 9)
		answer := strconv.Itoa(left + right)
		expiresAt := time.Now().Add(5 * time.Minute)

		http.SetCookie(w, &http.Cookie{
			Name:     "bookkeeping_captcha",
			Value:    makeCaptchaValue(answer, sessionSecret, expiresAt),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  expiresAt,
		})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"question": strconv.Itoa(left) + " + " + strconv.Itoa(right) + " = ?",
		})
	}
}

func isPublicPath(path string) bool {
	return path == "/captcha" || path == "/login" || path == "/logout" || path == "/healthz" || path == "/app-config" || strings.HasPrefix(path, "/assets/")
}

func isProtectedPage(path string, apiTestEnabled bool) bool {
	return path == "/" || path == "/orders" || (apiTestEnabled && path == "/api-test")
}

func isExternalOrderMutation(path, method string) bool {
	switch method {
	case http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	return strings.HasPrefix(path, "/api/v1/orders/")
}

func validAPIKey(r *http.Request, apiKey string) bool {
	provided := r.Header.Get("X-API-Key")
	if provided == "" {
		provided = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	return constantTimeEqual(provided, apiKey)
}

func validSession(r *http.Request, sessionSecret string) bool {
	cookie, err := r.Cookie("bookkeeping_session")
	if err != nil || cookie.Value == "" {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return false
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 3 {
		return false
	}
	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().After(time.Unix(expiresUnix, 0)) {
		return false
	}
	expected := sessionSignature(parts[0], parts[1], sessionSecret)
	return constantTimeEqual(parts[2], expected)
}

func makeSessionValue(username, sessionSecret string, expiresAt time.Time) string {
	expires := strconv.FormatInt(expiresAt.Unix(), 10)
	signature := sessionSignature(username, expires, sessionSecret)
	return base64.RawURLEncoding.EncodeToString([]byte(username + "|" + expires + "|" + signature))
}

func sessionSignature(username, expires, sessionSecret string) string {
	mac := hmac.New(sha256.New, []byte(sessionSecret))
	_, _ = mac.Write([]byte(username + "|" + expires))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func sessionCookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     "bookkeeping_session",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(12 * time.Hour),
	}
}

func expiredCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}

func makeCaptchaValue(answer, sessionSecret string, expiresAt time.Time) string {
	expires := strconv.FormatInt(expiresAt.Unix(), 10)
	signature := sessionSignature(answer, expires, sessionSecret)
	return base64.RawURLEncoding.EncodeToString([]byte(answer + "|" + expires + "|" + signature))
}

func validCaptcha(r *http.Request, sessionSecret, provided string) bool {
	cookie, err := r.Cookie("bookkeeping_captcha")
	if err != nil || cookie.Value == "" {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return false
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 3 {
		return false
	}
	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().After(time.Unix(expiresUnix, 0)) {
		return false
	}
	expected := sessionSignature(parts[0], parts[1], sessionSecret)
	return constantTimeEqual(parts[2], expected) && constantTimeEqual(strings.TrimSpace(provided), parts[0])
}

func randomInt(min, max int64) int {
	n, err := rand.Int(rand.Reader, big.NewInt(max-min+1))
	if err != nil {
		return int(min)
	}
	return int(n.Int64() + min)
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func safeNextURL(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/"
	}
	return value
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}
