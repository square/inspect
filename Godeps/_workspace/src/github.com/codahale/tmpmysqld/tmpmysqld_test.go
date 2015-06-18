package tmpmysql

import (
	"fmt"
	"reflect"
	"testing"
)

func Example() {
	if !IsMySQLInstalled() {
		panic("MySQL not installed")
	}

	server, err := NewMySQLServer("tmpmysqld_test")
	if err != nil {
		panic(err)
	}
	defer server.Stop()

	if _, err := server.DB.Exec(`
CREATE TABLE things (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(100) NOT NULL
)
`); err != nil {
		panic(err)
	}

	if _, err := server.DB.Exec(`
INSERT INTO things (name) VALUES ("one"), ("two")
`); err != nil {
		panic(err)
	}

	rows, err := server.DB.Query(`SELECT id, name FROM things ORDER BY id`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			panic(err)
		}
		fmt.Printf("%d=%s\n", id, name)
	}

	// Output:
	// 1=one
	// 2=two
}

func TestMySQLServer(t *testing.T) {
	if !IsMySQLInstalled() {
		t.Skip("MySQL not installed")
	}

	server, err := NewMySQLServer("tmpmysqld_test")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	if _, err := server.DB.Exec(`
CREATE TABLE things (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(100) NOT NULL
)
`); err != nil {
		t.Error(err)
	}

	if _, err := server.DB.Exec(`
INSERT INTO things (name) VALUES ("one"), ("two")
`); err != nil {
		t.Error(err)
	}

	rows, err := server.DB.Query(`SELECT id, name FROM things`)
	if err != nil {
		t.Error(err)
	}
	defer rows.Close()

	actual := make(map[int64]string)

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Error(err)
		}
		actual[id] = name
	}

	expected := map[int64]string{
		1: "one",
		2: "two",
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Was %#v, but expected %#v", actual, expected)
	}
}
