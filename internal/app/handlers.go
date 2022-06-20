package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"
)

var limiter = rate.NewLimiter(10, 30)

func limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Error represents a handler error. It provides methods for a HTTP status
// code and embeds the built-in error interface.
type Error interface {
	error
	Status() int
}

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Code int
	Err  error
}

// Allows StatusError to satisfy the error interface.
func (se StatusError) Error() string {
	return se.Err.Error()
}

// Returns our HTTP status code.
func (se StatusError) Status() int {
	return se.Code
}

func loggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Before executing the handler.
		start := time.Now()
		log.Printf("Strated %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		// After executing the handler.
		log.Printf("Completed %s in %v", r.URL.Path, time.Since(start))
	})
}

func basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(api.AuthUsername()))
			expectedPasswordHash := sha256.Sum256([]byte(api.AuthPassword()))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

type errorHandler func(http.ResponseWriter, *http.Request) error

func (f errorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := f(w, r)
	if err != nil {
		switch e := err.(type) {
		case Error:
			// We can retrieve the status here and write out a specific
			// HTTP status code.
			log.Printf("HTTP %d - %s", e.Status(), e)
			http.Error(w, e.Error(), e.Status())
		default:
			// Any error types we don't specifically look out for default
			// to serving a HTTP 500
			log.Printf("HTTP - %s", e)
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
		}
	}
}

// GetTransactionStatusHandler..
func GetTransactionStatusHandler() errorHandler {
	return func(rw http.ResponseWriter, r *http.Request) error {
		param := mux.Vars(r)["id"]
		id, err := strconv.Atoi(param)
		if err != nil {
			return &StatusError{http.StatusBadRequest, fmt.Errorf("error: id is NaN")}
		}
		st, err := api.GetTransactionStatus(id)
		if err != nil {
			return err
		}
		resp := map[string]interface{}{
			"id":                 id,
			"transaction_status": st,
		}
		data, _ := json.Marshal(resp)
		rw.Header().Add("content-type", "application/json")
		rw.WriteHeader(http.StatusFound)
		rw.Write(data)
		return nil
	}
}

// GetUserTransactionsHandler..
func GetUserTransactionsHandler() errorHandler {
	return func(rw http.ResponseWriter, r *http.Request) error {
		query := r.URL.Query()
		ts := make([]Transaction, 0)
		var err error
		if uid := query.Get("user_id"); uid != "" {
			userID, err := strconv.Atoi(uid)
			if err != nil {
				return &StatusError{http.StatusBadRequest, fmt.Errorf("error: 'user_id' is NaN")}
			}
			ts, err = api.GetUserTransactionsByID(userID)
			if err != nil {
				return err
			}
		} else if email := query.Get("email"); email != "" {
			ts, err = api.GetUserTransactionsByEmail(email)
			if err != nil {
				return err
			}
		} else {
			return &StatusError{
				http.StatusBadRequest,
				fmt.Errorf("error: no 'user_id' or 'email' provided"),
			}
		}

		order := query.Get("order")
		switch order {
		case "", "asc", "desc":
		default:
			return &StatusError{
				http.StatusBadRequest,
				fmt.Errorf("error: there is no order like '%s'; available orders: asc,desc", order),
			}
		}

		switch sorting := query.Get("sort"); {
		case sorting == "date" || sorting == "":
			if order == "desc" {
				sort.Slice(ts, func(i, j int) bool {
					return ts[i].ID > ts[j].ID
				})
			} else {
				sort.Slice(ts, func(i, j int) bool {
					return ts[i].ID < ts[j].ID
				})
			}
		case sorting == "amount":
			if order == "desc" {
				sort.Slice(ts, func(i, j int) bool {
					return ts[i].Amount > ts[j].Amount
				})
			} else {
				sort.Slice(ts, func(i, j int) bool {
					return ts[i].Amount < ts[j].Amount
				})
			}
		default:
			return &StatusError{
				http.StatusBadRequest,
				fmt.Errorf("error: there is no sort like '%s'; available sorts: date,amount", sorting),
			}
		}
		page := 0
		if val := query.Get("page"); val != "" {
			page, err = strconv.Atoi(val)
			if err != nil {
				return &StatusError{
					http.StatusBadRequest,
					fmt.Errorf("error: page should be integer, got '%v'", val),
				}
			}
		}

		start, end := Paginate(page, 10, len(ts))
		data, _ := json.Marshal(ts[start:end])
		rw.Header().Add("content-type", "application/json")
		rw.WriteHeader(http.StatusFound)
		rw.Write(data)
		return nil
	}
}

// CreateTransactionHandler..
func CreateTransactionHandler() errorHandler {
	return func(rw http.ResponseWriter, r *http.Request) error {
		t := new(Transaction)
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		err := decoder.Decode(t)
		if err != nil {
			return &StatusError{http.StatusBadRequest, err}
		}
		switch {
		case t.UserID == 0 || t.Email == "" || t.Amount == 0 || t.Currency == "":
			return &StatusError{http.StatusBadRequest, fmt.Errorf("error: required parameters: user_id, email, amount, currency")}
		case len([]rune(t.Email)) > 50:
			return &StatusError{http.StatusBadRequest, fmt.Errorf("error: email shouldn't be more than 50 characters")}
		case len([]rune(t.Currency)) > 20:
			return &StatusError{http.StatusBadRequest, fmt.Errorf("error: currency shouldn't be more than 20 characters")}
		}

		err = api.CreateTransaction(t)
		if err != nil {
			return err
		}
		resp := map[string]interface{}{
			"message":            "Added new Transaction",
			"id":                 t.ID,
			"transaction_status": t.Status,
		}
		data, _ := json.Marshal(resp)
		rw.WriteHeader(http.StatusCreated)
		rw.Write(data)
		return nil
	}
}

// ChangeTransactionStatusHandler..
func ChangeTransactionStatusHandler() errorHandler {
	return func(rw http.ResponseWriter, r *http.Request) error {
		t := new(Transaction)
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		err := decoder.Decode(t)
		if err != nil {
			return &StatusError{http.StatusBadRequest, err}
		}

		if t.ID == 0 || t.Status == "" {
			return &StatusError{http.StatusBadRequest, fmt.Errorf("error: required parameters: id, transaction_status")}
		}

		switch t.Status {
		case "УСПЕХ", "НЕУСПЕХ":
		default:
			return &StatusError{http.StatusBadRequest, fmt.Errorf("error: can't change transaction status to '%s'", t.Status)}
		}

		err = api.ChangeTransactionStatus(t.ID, t.Status)
		if err != nil {
			return err
		}
		resp := map[string]interface{}{
			"message": "Success! Status changed to " + t.Status,
			"id":      t.ID,
		}
		data, _ := json.Marshal(resp)
		rw.WriteHeader(http.StatusOK)
		rw.Write(data)
		return nil
	}
}

// CancelTransactionHandler..
func CancelTransactionHandler() errorHandler {
	return func(rw http.ResponseWriter, r *http.Request) error {
		param := mux.Vars(r)["id"]
		id, err := strconv.Atoi(param)
		if err != nil {
			return &StatusError{http.StatusBadRequest, fmt.Errorf("error: id is NaN")}
		}

		err = api.ChangeTransactionStatus(id, "ОТМЕНЕН")
		if err != nil {
			return err
		}
		resp := map[string]interface{}{
			"message": "Success! Transaction cancelled",
			"id":      id,
		}
		data, _ := json.Marshal(resp)
		rw.WriteHeader(http.StatusOK)
		rw.Write(data)
		return nil
	}
}

func Paginate(pageNum int, pageSize int, sliceLength int) (int, int) {
	start := pageNum * pageSize

	if start > sliceLength {
		start = sliceLength
	}

	end := start + pageSize
	if end > sliceLength {
		end = sliceLength
	}

	return start, end
}
