// Copyright (c) 2014 Square, Inc
//
// Tools to connect to and query the mysql database.
// Also includes parsers for "SHOW ENGINE INNODB" query.

package tools

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/square/inspect/Godeps/_workspace/src/code.google.com/p/goconf/conf" // used for parsing config files
)

// sql packages and driver
import "database/sql"
import _ "github.com/square/inspect/Godeps/_workspace/src/github.com/go-sql-driver/mysql" // mysql driver

type mysqlDB struct {
	db        *sql.DB
	dsnString string
}

const (
	// DefaultMySQLUser is the default user to use to connect to the database
	DefaultMySQLUser = "root"
	// MaxRetries is the number of retries made while connecting
	MaxRetries = 5
)

// Config represents configuration (password) required for client
type Config struct {
	Client struct {
		Password string
	}
}

// InnodbStats represents information about InnoDB
type InnodbStats struct {
	FileIO           map[string]string
	Log              map[string]string
	BufferPoolAndMem map[string]string
	Transactions     map[string]string
	Metrics          map[string]string
}

//wrapper for make_query, where if there is an error querying the database
// retry connecting to the db and make the query
func (database *mysqlDB) QueryDb(query string) ([]string, [][]string, error) {
	var err error
	for attempts := 0; attempts <= MaxRetries; attempts++ {
		err = database.db.Ping()
		if err == nil {
			if cols, data, err := database.makeQuery(query); err == nil {
				return cols, data, nil
			}
			return nil, nil, err
		}
		database.db.Close()
		database.db, err = sql.Open("mysql", database.dsnString)
	}
	return nil, nil, err
}

//wrapper for ExecQuery, which only returns a summary of the action taken
func (database *mysqlDB) DbExec(query string) (err error) {
	for attempts := 0; attempts <= MaxRetries; attempts++ {
		err = database.db.Ping()
		if err == nil {
			if _, err := database.db.Exec(query); err == nil {
				return nil
			}
			return err
		}
		database.db.Close()
		database.db, err = sql.Open("mysql", database.dsnString)
	}
	return err
}

//makes a query to the database
// returns array of column names and arrays of data stored as string
// string equivalent to []byte
// data stored as 2d array with each subarray containing a single column's data
func (database *mysqlDB) makeQuery(query string) ([]string, [][]string, error) {
	rows, err := database.db.Query(query)
	if err != nil {
		return nil, nil, err
	}

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	columns := len(columnNames)
	values := make([][]string, columns)
	tmpValues := make([]sql.RawBytes, columns)

	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &tmpValues[i]
	}

	for rows.Next() {
		err = rows.Scan(scanArgs...)
		if err != nil {
			return nil, nil, err
		}
		for i, col := range tmpValues {
			str := string(col)
			values[i] = append(values[i], str)
		}
	}
	err = rows.Err()
	if err != nil {
		return nil, nil, err
	}
	return columnNames, values, nil
}

func (database *mysqlDB) SetMaxConnections(maxConns int) {
	database.db.SetMaxOpenConns(maxConns)
}

// QueryReturnColumnDict returns values of query in a mapping of column_name -> column
func (database *mysqlDB) QueryReturnColumnDict(query string) (map[string][]string, error) {
	columnNames, values, err := database.QueryDb(query)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]string)
	for i, col := range columnNames {
		result[col] = values[i]
	}
	return result, nil
}

//return values of query in a mapping of first columns entry -> row
func (database *mysqlDB) QueryMapFirstColumnToRow(query string) (map[string][]string, error) {
	_, values, err := database.QueryDb(query)
	result := make(map[string][]string)
	if len(values) == 0 {
		return nil, nil
	}
	for i, name := range values[0] {
		for j, vals := range values {
			if j != 0 {
				result[string(name)] = append(result[string(name)], vals[i])
			}
		}
	}
	return result, err
}

//makes dsn to open up connection
//dsn is made up of the format:
//     [user[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
func makeDsn(dsn map[string]string) string {
	var dsnString string
	user, userok := dsn["user"]
	if userok {
		dsnString = user
	}
	password, ok := dsn["password"]
	if ok {
		dsnString = dsnString + ":" + password
	}
	if userok {
		dsnString = dsnString + "@"
	}
	dsnString = dsnString + dsn["host"]
	dsnString = dsnString + "/" + dsn["dbname"]
	dsnString = dsnString + "?timeout=30s"
	return dsnString
}

// New create connection to mysql database here
// if an error is encountered, still return database so that the logger may be used
func New(user, password, host, config string) (MysqlDB, error) {

	dsn := map[string]string{"dbname": "information_schema"}
	creds := map[string]string{"root": "/root/.my.cnf", "nrpe": "/etc/my_nrpe.cnf"}

	database := &mysqlDB{}

	if user == "" {
		user = DefaultMySQLUser
		dsn["user"] = DefaultMySQLUser
	} else {
		dsn["user"] = user
	}
	if password != "" {
		dsn["password"] = password
	}

	// ex: "unix(/var/lib/mysql/mysql.sock)"
	// ex: "tcp(your.db.host.com:3306)"
	dsn["host"] = host

	//Parse ini file to get credentials
	iniFile := creds[user]
	if config != "" {
		iniFile = config
	}
	_, err := os.Stat(iniFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return database, errors.New("'" + iniFile + "' does not exist")
	}
	// read ini file
	c, err := conf.ReadConfigFile(iniFile)
	if err != nil {
		return database, err
	}

	// Override the username if specified
	iniUser, err := c.GetString("client", "user")
	if err == nil {
		dsn["user"] = iniUser
	}

	// Set the password
	pw, err := c.GetString("client", "password")
	dsn["password"] = strings.Trim(pw, " \"")

	// Build the DSN
	database.dsnString = makeDsn(dsn)

	//make connection to db
	db, err := sql.Open("mysql", database.dsnString)
	if err != nil {
		return database, err
	}
	database.db = db

	//ping db to verify connection
	err = database.db.Ping()
	if err != nil {
		return database, err
	}
	return database, nil
}

func (database *mysqlDB) Log(in interface{}) {
	_, f, line, ok := runtime.Caller(1)
	if ok {
		log.Println("Log from: " + f + " line: " + strconv.Itoa(line))
	}
	log.Println(in)
}

func (database *mysqlDB) Close() {
	database.db.Close()
}

// ParseInnodbStats parses results from "SHOW ENGINE INNODB STATUS" query
func ParseInnodbStats(blob string) (*InnodbStats, error) {
	idb := new(InnodbStats)
	idb.Metrics = make(map[string]string)

	chunks := regexp.MustCompile("\n[-=]{3,80}\n").Split(blob, -1)
	for i, chunk := range chunks {
		m := regexp.MustCompile("([/ A-Z])\\s*$").MatchString(chunk)
		if m {
			chunk = strings.Trim(chunk, " \n")
			if m, _ := regexp.MatchString("FILE I/O", chunk); m {
				idb.parseFileIO(chunks[i+1])
			} else if chunk == "LOG" {
				idb.parseLog(chunks[i+1])
			} else if chunk == "BUFFER POOL AND MEMORY" {
				idb.parseBufferPoolAndMem(chunks[i+1])
			} else if chunk == "TRANSACTIONS" {
				idb.parseTransactions(chunks[i+1])
			}
		}
	}
	return idb, nil
}

//parse the File I/O section of the "show engine innodb status;" command
//stores expressions of the form:     123.456 metric_name
func (idb *InnodbStats) parseFileIO(blob string) {
	lines := strings.Split(blob, "\n")
	for _, line := range lines {
		if strings.Contains(line, ",") {
			elements := strings.Split(line, ",")
			for _, element := range elements {
				element = strings.Trim(element, " \n")
				m := regexp.MustCompile("^(\\d+(\\.\\d+)?) ([A-Za-z/ ]+)\\s*$").FindStringSubmatch(element)
				if len(m) == 4 {
					key := strings.Replace(strings.Replace(m[3], " ", "_", -1), "/", "_per_", -1)
					idb.Metrics[key] = m[1]
				}
			}
		}
	}
}

//parse the log section of the "show engine innodb status;" command
func (idb *InnodbStats) parseLog(blob string) {
	lines := strings.Split(blob, "\n")
	for _, line := range lines {
		line := strings.Trim(line, " \n")
		if regexp.MustCompile("^(.+?)\\s+(\\d+)\\s*$").MatchString(line) {
			elements := strings.Split(line, " ")
			c := len(elements)
			val := elements[c-1]
			key := strings.Trim(strings.ToLower(strings.Join(elements[:c-1], "_")), "_")
			idb.Metrics[key] = val
		} else {
			elements := strings.Split(line, ",")
			for _, element := range elements {
				element = strings.Trim(element, " \n\t\r\f")
				if regexp.MustCompile("(\\d+) ([A-Za-z/ ,']+)\\s*$").MatchString(element) {
					element = strings.Replace(strings.Replace(element, "i/o's", "io", -1), "/second", "_per_sec", -1)
					words := strings.Split(element, " ")
					key := strings.Trim(strings.ToLower(strings.Join(words[1:], "_")), "_")
					idb.Metrics[key] = words[0]
				}
			}
		}
	}
}

func (idb *InnodbStats) parseBufferPoolAndMem(blob string) {
	lines := strings.Split(blob, "\n")
	matches := []string{"Page hash", "Dictionary cache", "File system", "Lock system", "Recovery system",
		"Dictionary memory allocated", "Buffer pool size", "Free buffers", "Database pages", "Old database pages",
		"Modified db pages", "Pending reads", "Pending writes: LRU", "Pages made young", "Pages read"}
	for _, line := range lines {
		line = strings.Split(strings.Trim(line, " \n"), ",")[0]
		//so many regular expressions. just gonna hard code some of them
		words := strings.Split(line, " ")
		if m, _ := regexp.MatchString("Total memory allocated by read views \\d+", line); m {
			idb.Metrics["total_mem_by_read_views"] = words[len(words)-1]
		} else if m, _ := regexp.MatchString("Total memory allocated \\d+", line); m {
			line := strings.Split(line, ";")[0]
			words := strings.Split(line, " ")
			idb.Metrics["total_mem"] = words[len(words)-1]
		} else if m, _ := regexp.MatchString("Adaptive hash index", line); m {
			idb.Metrics["adaptive_hash"] = words[3]
		} else {
			for _, match := range matches {
				if m, _ := regexp.MatchString(match, line); m {
					line = strings.Split(line, ",")[0]
					key := strings.Trim(strings.ToLower(strings.Replace(strings.Replace(match, " ", "_", -1), ":", "", -1)), " \n\t\f\r")
					if _, ok := idb.Metrics[key]; ok {
						continue
					}
					keyLen := len(strings.Split(key, "_"))
					idb.Metrics[key] = strings.Trim(strings.Split(strings.Join(words[keyLen:], ""), "(")[0], " \n\t\f\r")
				} else if m, _ := regexp.MatchString("Buffer pool hit rate", line); m {
					line := strings.Split(line, ",")[0]
					words := strings.Split(line, " ")
					num, _ := strconv.ParseFloat(words[4], 64)
					den, _ := strconv.ParseFloat(words[6], 64)
					idb.Metrics["buffer_pool_hit_rate"] = strconv.FormatFloat(num/den, 'f', -1, 64)
					idb.Metrics["cache_hit_pct"] = strconv.FormatFloat((num/den)*100.0, 'f', -1, 64)
				}
			}
		}

	}
}

func (idb *InnodbStats) parseTransactions(blob string) {
	trxesNotStarted := 0
	undo := 0
	lines := strings.Split(blob, "\n")
	rollbackexpr := "^ROLLING BACK \\d+ lock struct\\(s\\), heap size \\d+, \\d+ row lock\\(s\\), undo log entries (\\d+)"
	for _, line := range lines {
		line = strings.Trim(line, " ")
		if m := regexp.MustCompile(rollbackexpr).FindStringSubmatch(line); len(m) > 0 {
			tmp, _ := strconv.Atoi(m[1])
			if tmp > undo {
				undo = tmp
			}
		} else if regexp.MustCompile("^(.+?)\\s+(\\d+)\\s*$").MatchString(line) {
			words := strings.Split(line, " ")
			key := strings.ToLower(strings.Join(words[:len(words)-2], "_"))
			idb.Metrics[key] = words[len(words)-1]
		} else if m, _ := regexp.MatchString("TRANSACTION (.*) not started", line); m {
			trxesNotStarted++
		}
	}
	idb.Metrics["trxes_not_started"] = strconv.Itoa(trxesNotStarted)
	idb.Metrics["undo"] = strconv.Itoa(undo)
}
