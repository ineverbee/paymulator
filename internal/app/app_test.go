package app

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

type MockServer struct{}

func (ms *MockServer) AuthUsername() string {
	return "username"
}
func (ms *MockServer) AuthPassword() string {
	return "password"
}

func (ms *MockServer) GetTransactionStatus(id int) (string, error) {
	if id < 0 {
		return "", fmt.Errorf("Internal Server Error")
	}
	return "", nil
}

func (ms *MockServer) GetUserTransactionsByID(id int) ([]Transaction, error) {
	return []Transaction{{ID: 1, Amount: 1.2}, {ID: 2, Amount: 11.2}}, nil
}

func (ms *MockServer) GetUserTransactionsByEmail(email string) ([]Transaction, error) {
	return []Transaction{{ID: 1, Amount: 1.2}, {ID: 2, Amount: 11.2}}, nil
}

func (ms *MockServer) CreateTransaction(t *Transaction) error {
	return nil
}

func (ms *MockServer) ChangeTransactionStatus(id int, st string) error {
	return nil
}

func TestHandlers(t *testing.T) {
	api = &MockServer{}
	router := mux.NewRouter()

	// Check transaction status
	router.Handle("/transactions/{id}", loggingHandler(limit(errorHandler(GetTransactionStatusHandler())))).Methods("GET")
	// Get Uset transactions by ID or email
	router.Handle("/transactions", loggingHandler(limit(errorHandler(GetUserTransactionsHandler())))).Methods("GET")
	// Create transaction
	router.Handle("/transaction", loggingHandler(limit(errorHandler(CreateTransactionHandler())))).Methods("POST")
	// Change transaction status
	router.Handle("/transaction", loggingHandler(limit(basicAuth(errorHandler(ChangeTransactionStatusHandler()))))).Methods("PUT")
	// Cancel transaction
	router.Handle("/transaction/{id}", loggingHandler(limit(errorHandler(CancelTransactionHandler())))).Methods("PUT")

	tc := []struct {
		method, target string
		body           io.Reader
		code           int
	}{
		{"GET", "/", nil, http.StatusNotFound},
		{"GET", "/transactions/1", nil, http.StatusFound},
		{"GET", "/transactions/-1", nil, http.StatusInternalServerError},
		{"GET", "/transactions/", nil, http.StatusNotFound},
		{"GET", "/transactions/smth", nil, http.StatusBadRequest},
		{"GET", "/transactions?user_id=1&order=asc", nil, http.StatusFound},
		{"GET", "/transactions?user_id=1&order=wrong", nil, http.StatusBadRequest},
		{"GET", "/transactions?user_id=NaN", nil, http.StatusBadRequest},
		{"GET", "/transactions?email=example@mail.com", nil, http.StatusFound},
		{"GET", "/transactions", nil, http.StatusBadRequest},
		{"GET", "/transactions?user_id=1&sort=user", nil, http.StatusBadRequest},
		{"GET", "/transactions?email=example@mail.com&sort=date&order=desc", nil, http.StatusFound},
		{"GET", "/transactions?user_id=1&order=badinput", nil, http.StatusBadRequest},
		{"GET", "/transactions?user_id=1&sort=amount&order=asc&page=100", nil, http.StatusFound},
		{"GET", "/transactions?email=example@mail.com&sort=amount&order=desc&page=NaN", nil, http.StatusBadRequest},
		{"POST", "/transaction", strings.NewReader("{\"user_id\": 1, \"email\": \"exmpl@m.com\", \"amount\": 1.5, \"currency\": \"USD\"}"), http.StatusCreated},
		{"POST", "/transaction", strings.NewReader(fmt.Sprintf("{\"user_id\": 1, \"email\": \"%s\", \"amount\": 1.5, \"currency\": \"USD\"}", strings.Repeat("a", 51))), http.StatusBadRequest},
		{"POST", "/transaction", strings.NewReader(fmt.Sprintf("{\"user_id\": 1, \"email\": \"exmpl@m.com\", \"amount\": 1.5, \"currency\": \"%s\"}", strings.Repeat("a", 21))), http.StatusBadRequest},
		{"POST", "/transaction", strings.NewReader("{\"user_id\": 1, \"email\": \"exmpl@m.com\", \"amount\": 1.5}"), http.StatusBadRequest},
		{"POST", "/transaction", strings.NewReader("no]/:fie;OeFM"), http.StatusBadRequest},
		{"PUT", "/transaction", strings.NewReader("{\"id\": 1, \"transaction_status\": \"УСПЕХ\"}"), http.StatusUnauthorized},
		{"PUT", "/transaction/1", nil, http.StatusOK},
		{"PUT", "/transaction/smth", nil, http.StatusBadRequest},
	}
	for _, c := range tc {
		request(t, router, c.method, c.target, c.body, c.code)
	}

	tc = []struct {
		method, target string
		body           io.Reader
		code           int
	}{
		{"PUT", "/transaction", strings.NewReader("{\"id\": 1, \"transaction_status\": \"УСПЕХ\"}"), http.StatusOK},
		{"PUT", "/transaction", strings.NewReader("{\"id\": 1, \"transaction_status\": \"SUCCESS\"}"), http.StatusBadRequest},
		{"PUT", "/transaction", strings.NewReader("{\"transaction_status\": \"НЕУСПЕХ\"}"), http.StatusBadRequest},
		{"PUT", "/transaction", strings.NewReader("BadIn*p|ut"), http.StatusBadRequest},
	}

	for _, c := range tc {
		req := httptest.NewRequest(c.method, c.target, c.body)
		req.SetBasicAuth("username", "password")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		require.Equal(t, c.code, rr.Code)
	}

	req := httptest.NewRequest("GET", "/transactions/1", nil)
	rr := httptest.NewRecorder()
	for i := 0; i < 40; i++ {
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)
	}
	require.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func request(t *testing.T, handler http.Handler, method, target string, body io.Reader, code int) {
	req := httptest.NewRequest(method, target, body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, code, rr.Code)
}
