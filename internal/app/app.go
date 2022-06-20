package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Server interface {
	AuthUsername() string
	AuthPassword() string

	CreateTransaction(*Transaction) error
	GetTransactionStatus(int) (string, error)
	GetUserTransactionsByID(int) ([]Transaction, error)
	GetUserTransactionsByEmail(string) ([]Transaction, error)
	ChangeTransactionStatus(int, string) error
}

type ApiServer struct {
	server   http.Server
	database *pgxpool.Pool
	auth     struct {
		username string
		password string
	}
}

var api Server
var timeout = 30 * time.Second

func StartServer() error {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?connect_timeout=5",
		os.Getenv("DB_USERNAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	dbConn, err := NewDB(ctx, connStr)
	if err != nil {
		return err
	}

	api = &ApiServer{
		http.Server{Addr: ":8080", Handler: router},
		dbConn,
		struct {
			username string
			password string
		}{
			os.Getenv("PAYMENT_SYSTEM_USERNAME"),
			os.Getenv("PAYMENT_SYSTEM_PASSWORD"),
		},
	}

	log.Println("Staring server on Port 8080")
	err = http.ListenAndServe(":8080", router)
	return err
}

func NewDB(ctx context.Context, connStr string) (*pgxpool.Pool, error) {
	log.Printf("Trying to connect to %s\n", connStr)
	var (
		conn *pgxpool.Pool
		err  error
	)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutExceeded := time.After(timeout)
LOOP:
	for {
		select {
		case <-timeoutExceeded:
			return nil, fmt.Errorf("db connection failed after %s timeout", timeout)

		case <-ticker.C:
			conn, err = pgxpool.Connect(ctx, connStr)
			if err == nil {
				break LOOP
			}
			log.Println("Failed! Trying to reconnect..")
		}
	}

	err = conn.Ping(ctx)
	if err != nil {
		return nil, err
	}

	log.Println("Connect success!")

	return conn, nil
}

func (s *ApiServer) AuthUsername() string {
	return s.auth.username
}

func (s *ApiServer) AuthPassword() string {
	return s.auth.password
}

func (s *ApiServer) GetTransactionStatus(id int) (string, error) {
	st := ""
	q := "SELECT transaction_status FROM transactions WHERE id=" + fmt.Sprint(id)
	err := pgxscan.Get(context.TODO(), s.database, &st, q)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", &StatusError{http.StatusNotFound, fmt.Errorf("error: transaction not found")}
		}
		return "", err
	}
	return st, nil
}

func (s *ApiServer) GetUserTransactionsByID(id int) ([]Transaction, error) {
	var l int
	err := s.database.QueryRow(context.TODO(), "select count(*) from transactions where user_id="+fmt.Sprint(id)).Scan(&l)
	if err != nil {
		return nil, err
	}

	ts := make([]Transaction, l)
	rows, err := s.database.Query(
		context.TODO(),
		"select * from transactions where user_id="+fmt.Sprint(id),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for i := 0; rows.Next(); i++ {
		rows.Scan(
			&ts[i].ID,
			&ts[i].UserID,
			&ts[i].Email,
			&ts[i].Amount,
			&ts[i].Currency,
			&ts[i].Created_at,
			&ts[i].Changed_at,
			&ts[i].Status,
		)
	}
	return ts, nil
}

func (s *ApiServer) GetUserTransactionsByEmail(email string) ([]Transaction, error) {
	var l int
	err := s.database.QueryRow(
		context.TODO(),
		fmt.Sprintf("select count(*) from transactions where email='%s'", email),
	).Scan(&l)
	if err != nil {
		return nil, err
	}

	ts := make([]Transaction, l)
	rows, err := s.database.Query(
		context.TODO(),
		fmt.Sprintf("select * from transactions where email='%s'", email),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for i := 0; rows.Next(); i++ {
		rows.Scan(
			&ts[i].ID,
			&ts[i].UserID,
			&ts[i].Email,
			&ts[i].Amount,
			&ts[i].Currency,
			&ts[i].Created_at,
			&ts[i].Changed_at,
			&ts[i].Status,
		)
	}
	return ts, nil
}

func (s *ApiServer) CreateTransaction(t *Transaction) error {
	rand.Seed(time.Now().UTC().UnixNano())
	switch rand.Intn(2) {
	case 0:
		t.Status = "НОВЫЙ"
	case 1:
		t.Status = "ОШИБКА"
	}
	err := s.database.QueryRow(
		context.TODO(),
		"insert into transactions (user_id, email, amount, currency, transaction_status) values ($1,$2,$3,$4,$5) returning id",
		t.UserID,
		t.Email,
		t.Amount,
		t.Currency,
		t.Status,
	).Scan(&t.ID)
	if err != nil {
		return err
	}
	return nil
}

func (s *ApiServer) ChangeTransactionStatus(id int, st string) error {
	status := ""
	q := "SELECT transaction_status FROM transactions WHERE id=" + fmt.Sprint(id)
	err := pgxscan.Get(context.TODO(), s.database, &status, q)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &StatusError{http.StatusBadRequest, err}
		}
		return err
	}
	switch status {
	case "УСПЕХ", "НЕУСПЕХ":
		return &StatusError{http.StatusConflict, fmt.Errorf("error: status '%s' can't be changed", status)}
	case "ОТМЕНЕН":
		if st == "ОТМЕНЕН" {
			return &StatusError{http.StatusConflict, fmt.Errorf("error: status already '%s'", status)}
		}
	}
	_, err = s.database.Exec(
		context.TODO(),
		fmt.Sprintf("update transactions set transaction_status='%s', changed_at=CURRENT_TIMESTAMP where id=%d", st, id),
	)
	if err != nil {
		return err
	}
	return nil
}
