# Movie Technical Test

## Description

This is a project built with Golang Clean architecture. This project is an api service for movie management.

## Tech Stack

- Golang v1.20
- MySQL (Database) v5.7

## Framework & Library

- GoFiber (HTTP Framework) : https://github.com/gofiber/fiber
- GORM (ORM) : https://github.com/go-gorm/gorm
- Viper (Configuration) : https://github.com/spf13/viper
- Golang Migrate (Database Migration) : https://github.com/golang-migrate/migrate
- Go Playground Validator (Validation) : https://github.com/go-playground/validator
- Logrus (Logger) : https://github.com/sirupsen/logrus

## Configuration

All configuration is in `config.json` file.

## API Spec

All API collection is in `api` folder. Recommended import to Postman.

JSON Response status:
| Status        | Description                   | HTTP Status code|
| ------------- |:-----------------------------:| --------------- |
| 99            | Internal server error         | 500             |
| 00            | Operation success             | 200             |
| 01            | Data not found in database    | 200             |
| 04            | request validation error      | 400             |


## Database Migration

All database sql is in `db` folder. mysql will be installed automatically when using docker compose.

## Run Application

### Using docker compose

Go to the project root directory and execute the command below:
```shell
docker-compose up -d
```
Check application running in browser
```shell
http://localhost:8080/ping
```