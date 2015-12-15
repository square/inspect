package tools

// MysqlDB interface defines few common methods used
type MysqlDB interface {
	// set the max number of database connections allowed at once
	SetMaxConnections(maxConns int)

	// makes query to database
	// returns result as a mapping of strings to string arrays
	// where key is column name and value is the items stored in column
	// in same order as rows
	QueryReturnColumnDict(query string) (map[string][]string, error)

	// makes query to database
	// returns result as a mapping of strings to string arrays
	// where key is the value stored in the first column of a row
	// and is mapped to the remaining values in the row
	// in the order as they appeared in the row
	QueryMapFirstColumnToRow(query string) (map[string][]string, error)

	// Log Prints in to the logger
	Log(in interface{})

	//wrapper for make_query, where if there is an error querying the database
	// retry connecting to the db and make the query
	QueryDb(query string) ([]string, [][]string, error)

	//wrapper for ExecQuery, which only returns a summary of the action taken
	DbExec(query string) (err error)

	// Closes the connection with the database
	Close()
}
