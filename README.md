
# paymulator

### Payment Service Emulator

The service accepts requests via the REST API, saves/changes payment statuses in the database.


## Badges

[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/ineverbee/paymulator.svg)](https://github.com/ineverbee/paymulator) 
[![Go Report Card](https://goreportcard.com/badge/github.com/ineverbee/paymulator)](https://goreportcard.com/report/github.com/ineverbee/paymulator) 
![gopherbadger-tag-do-not-edit](coverage_badge.png)
## Run Locally

Clone the project

```bash
  git clone https://github.com/ineverbee/paymulator.git
```

Go to the project directory

```bash
  cd paymulator
```

Start the server

```bash
  docker-compose up
```


## Running Tests

To run tests, run the following command

```bash
  go test -v -cover ./...
```
There is also Postman collection `paymulator.postman_collection.json` to run some other requests

## Environment Variables

Folowing environment variables already added to docker-compose.yml

`DB_USERNAME: "pguser"`

`DB_PASSWORD: "pgpwd4"`

`DB_HOST: "postgres"`

`DB_PORT: "5432"`

`DB_NAME: "test_db"`

These two env variables used for authorization

`PAYMENT_SYSTEM_USERNAME: "kiwi"`

`PAYMENT_SYSTEM_PASSWORD: "p8fnxeqj5a7zbrqp"`


## API Reference

#### Get User Transactions By ID/Email

```http
  GET /transactions
```

| Query Parameter | Type     | Description                |
| :-------- | :------- | :------------------------- |
| `user_id` | `int`    | **Required**. One of the two `user_id / email` |
| `email`   | `string` | **Required**. One of the two `user_id / email` |
| `order`   | `string` | *Optional*. `asc / desc` - ascending / descending order |
| `sort`    | `string` | *Optional*. `date / amount` - sort by creation date / amount |
| `page`    | `int`    | *Optional*. `0,1,2..` - pages |

Example: `/transactions?user_id=1&sort=amount&order=asc&page=10`

#### Get Transaction Status

```http
  GET /transactions/{id}
```

| Parameter | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `id`      | `int`    | **Required**. Id of transaction to check |

Example: `/transactions/1`

#### Create Transaction

```http
  POST /transaction
```

| Body Parameter (JSON) | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `user_id`      | `int`    | **Required**. Id of the user creating transaction |
| `email`        | `string` | **Required**. Email of the user creating transaction |
| `amount`       | `float`  | **Required**. Transaction amount |
| `currency`     | `string` | **Required**. Transaction currency |

Example cURL request:
```bash
curl --header "Content-Type: application/json" \
  --request POST \
  --data '{
        "user_id": 12,
        "email": "lol@gmail.com",
        "amount": 800.00,
        "currency": "USD"
    }' \
  http://localhost:8080/transaction
```
#### Change Transaction Status

```http
  PUT /transaction
```

| Body Parameter (JSON) | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `id`      | `int`    | **Required**. Id of transaction to change |
| `transaction_status`      | `string`    | **Required**. Status to change to |

| Basic Auth | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `username`      | `string`    | **Required**. Username for authorization |
| `password`      | `string`    | **Required**. Password for authorization |

Example cURL request: 

```bash
curl --header "Content-Type: application/json" \
  --request PUT \
  --user kiwi:p8fnxeqj5a7zbrqp \
  --data '{
        "id": 1,
        "transaction_status": "НЕУСПЕХ"
    }' \
  http://localhost:8080/transaction
```

#### Cancel Transaction

```http
  PUT /transaction/{id}
```

| Parameter | Type     | Description                       |
| :-------- | :------- | :-------------------------------- |
| `id`      | `int`    | **Required**. Id of transaction to cancel |

Example cURL request: 

```bash
curl --header "Content-Type: application/json" \
  --request PUT \
  http://localhost:8080/transaction/1
```
