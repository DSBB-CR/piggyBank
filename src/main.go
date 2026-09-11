package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var tmpl = template.Must(template.New("").Funcs(template.FuncMap{
	"money": formatMoney,
}).ParseGlob("templates/*.html"))

func main() {
	store := NewStore("data.json")
	if err := store.Load(); err != nil {
		log.Println("load:", err)
	}

	mux := http.NewServeMux()

	// API
	mux.HandleFunc("GET /api/accounts", apiList(store))
	mux.HandleFunc("POST /api/accounts", apiCreate(store))
	mux.HandleFunc("DELETE /api/accounts/{id}", apiDeleteAccount(store))
	mux.HandleFunc("POST /api/accounts/{id}/transactions", apiAddTx(store))
	mux.HandleFunc("DELETE /api/accounts/{id}/transactions/{txid}", apiDeleteTx(store))

	// HTML
	mux.HandleFunc("GET /{$}", pageIndex(store))
	mux.HandleFunc("GET /accounts/{id}", pageAccount(store))

	// Static
	mux.Handle("GET /static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static"))))

	log.Println("→ http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// ---------- HTML ----------

func pageIndex(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accounts := store.Accounts()
		var total float64
		for _, a := range accounts {
			total += a.Balance()
		}
		render(w, "layout.html", map[string]any{
			"Accounts":   accounts,
			"Total":      total,
			"Currencies": []string{"RUB", "USD", "EUR", "KZT", "BYN"},
		})
	}
}

func pageAccount(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		acc, ok := store.Account(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		render(w, "layout2.html", map[string]any{
			"Account": acc,
		})
	}
}

func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Println("template:", err)
	}
}

// ---------- API ----------

func apiList(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, store.Accounts())
	}
}

func apiCreate(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name     string  `json:"name"`
			Currency string  `json:"currency"`
			Initial  float64 `json:"initial"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, errJSON("неверный JSON"))
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			writeJSON(w, 400, errJSON("укажите название"))
			return
		}
		if req.Currency == "" {
			req.Currency = "RUB"
		}
		acc := store.CreateAccount(req.Name, req.Currency, req.Initial)
		writeJSON(w, 201, acc)
	}
}

func apiDeleteAccount(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !store.DeleteAccount(r.PathValue("id")) {
			writeJSON(w, 404, errJSON("не найдено"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func apiAddTx(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Amount float64 `json:"amount"`
			Note   string  `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, errJSON("неверный JSON"))
			return
		}
		if req.Amount == 0 {
			writeJSON(w, 400, errJSON("сумма не может быть 0"))
			return
		}
		tx, ok := store.AddTransaction(r.PathValue("id"), req.Amount, req.Note)
		if !ok {
			writeJSON(w, 404, errJSON("счёт не найден"))
			return
		}
		writeJSON(w, 201, tx)
	}
}

func apiDeleteTx(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !store.DeleteTransaction(r.PathValue("id"), r.PathValue("txid")) {
			writeJSON(w, 404, errJSON("не найдено"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errJSON(msg string) map[string]string { return map[string]string{"error": msg} }

func formatMoney(v float64, currency string) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	// разделим тысячи
	parts := strings.Split(s, ".")
	intPart := parts[0]
	neg := strings.HasPrefix(intPart, "-")
	if neg {
		intPart = intPart[1:]
	}
	var out []byte
	for i, c := range []byte(intPart) {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	if neg {
		intPart = "-" + string(out)
	} else {
		intPart = string(out)
	}
	sym := map[string]string{"RUB": "₽", "USD": "$", "EUR": "€", "KZT": "₸", "BYN": "Br"}[currency]
	if sym == "" {
		sym = currency
	}
	return intPart + "." + parts[1] + " " + sym
}
