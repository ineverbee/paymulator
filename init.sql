DROP DATABASE IF EXISTS test_db;    

CREATE DATABASE test_db;    

\c test_db; 

CREATE TYPE choice AS ENUM ('НОВЫЙ', 'УСПЕХ', 'НЕУСПЕХ', 'ОШИБКА', 'ОТМЕНЕН');

CREATE TABLE transactions (
    id SERIAL NOT NULL PRIMARY KEY,
    user_id INT,
    email VARCHAR,
    amount FLOAT,
    currency VARCHAR,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    transaction_status choice
);