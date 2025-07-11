package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func getConn(name string) *sql.DB {
    os.Mkdir("databases", 0644)
    db, err := sql.Open("sqlite3", "databases/"+name)
    if err != nil {
        fmt.Println(err)
		return nil
    }

    query1 := "CREATE TABLE IF NOT EXISTS main (x TEXT PRIMARY KEY, y TEXT)"
    _, err = db.Exec(query1)
    if err != nil {
        fmt.Println(err)
		return nil
    }
    return db
}

func Set(db *sql.DB, key string, val string) {
    queryCheck := "SELECT x FROM main WHERE x = ?"
    var existingKey string
    err := db.QueryRow(queryCheck, key).Scan(&existingKey)

    if err == sql.ErrNoRows {
        queryInsert := "INSERT INTO main (x, y) VALUES (?, ?)"
        _, err = db.Exec(queryInsert, key, val)
        if err != nil {
            fmt.Println(err)
			return
        }
    } else if err != nil {
		fmt.Println(err)
		return
    } else {
        queryUpdate := "UPDATE main SET y = ? WHERE x = ?"
        _, err = db.Exec(queryUpdate, val, key)
        if err != nil {
            fmt.Println(err)
			return
        }
    }
}

func Get(db *sql.DB, key string) (string, bool) {
    query := "SELECT y FROM main WHERE x = ?"
    var value string
    err := db.QueryRow(query, key).Scan(&value)
    if err == sql.ErrNoRows {
        return "", false
    } else if err != nil {
		fmt.Println(err)
		return "", false
    }
    return value, true
}

func Get_All(db *sql.DB, key string) (map[string]string, bool) {
    query := "SELECT x, y FROM main WHERE x LIKE ?"
    rows, err := db.Query(query, key+"%")
    if err != nil {
        fmt.Println(err)
        return nil, false
    }
    defer rows.Close()

    values := make(map[string]string)
    for rows.Next() {
        var k, v string
        if err := rows.Scan(&k, &v); err != nil {
            fmt.Println(err)
            return nil, false
        }
        values[k] = v
    }
    if err := rows.Err(); err != nil {
        fmt.Println(err)
        return nil, false
    }

    if len(values) == 0 {
        return nil, false
    }
    return values, true
}

func Delete(db *sql.DB, key string) bool {
    query := "DELETE FROM main WHERE x = ?"
    result, err := db.Exec(query, key)
    if err != nil {
        fmt.Println(err)
        return false
    }
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        fmt.Println(err)
        return false
    }
    return rowsAffected > 0
}