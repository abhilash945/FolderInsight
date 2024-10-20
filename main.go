//go:build windows || !windows
// +build windows !windows

package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Global variables
var (
	dirPath                string
	DBfile                 string
	updateErrorOnly        bool
	debug                  bool
	updateWindowsFileOwner bool
	channelSize            int
	insertionBatchSizeSQL  = 200 // Number of rows to insert in one query
	infoMultiLogger        *log.Logger
	errorMultiLogger       *log.Logger
	infoFileLogger         *log.Logger
	// readFolderCounter     int32                       // Atomic counter for active goroutines
	sem = make(chan struct{}, 8000) // semaphore used to set max goroutines
)

// Represents the scan data gathered and stored to DB
type ObjectInfo struct {
	ObjType            string // d- directory, f- file, l- link, o- other
	Path               string
	ObjectDepth        int
	FileSize           int //size of a file
	ThisFolderSize     int //folder size with all the containing files only
	TotalCalFolderSize int //Total folder size(with all the containing files & subfolders)
	hasError           bool
	ErrorMessage       string
	Owner              string
	CreationTime       time.Time
	LastWriteTime      time.Time
	CalLastWriteTime   time.Time
	LastAccessTime     time.Time
	// CreatedBy        string // Placeholder, platform-specific implementation needed
	// LastModifiedBy   string // Placeholder, platform-specific implementation needed
	// SubObjects       []ObjectInfo
	// NumSubFiles      int
	// NumSubFolders    int
}

// Represents the failed folder list if updateErrorOnly is enabled
type ErrorObjectInfo struct {
	Path        string
	ObjectDepth int
}

// FolderInfoCal struct holds folder information
type FolderInfoCal struct {
	TotalCalFolderSize int
	CalLastWriteTime   time.Time
}

// starts here
func main() {
	preCheckErrors := false //assume as no precheck errors
	// Define flags
	flag.StringVar(&dirPath, "Path", "", "Folder to scan (mandatory)")
	flag.StringVar(&DBfile, "DBfile", "", "Result report DB file (mandatory)")
	flag.IntVar(&channelSize, "BufferSize", 100000, "meta data buffer size (optional)")
	flag.IntVar(&insertionBatchSizeSQL, "SQLBatchSize", 200, "DB batch size for buffered insertions (optional)")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging (optional, default is false)")
	flag.BoolVar(&updateErrorOnly, "UpdateErrorOnly", false, "Run scan only on failed directories (optional, default is false)")
	flag.BoolVar(&updateWindowsFileOwner, "UpdateWindowsFileOwner", false, "Update the file owner or creater name (optional, default is false, applicable in windows only)")
	// Parse provided flags
	flag.Parse()

	//check if the mandatory fields are missing
	if dirPath == "" || DBfile == "" {
		fmt.Println("Mandatory fields are missing, check with -help")
		os.Exit(0)
	}

	//check if the directory is a valid one
	if info, err := os.Stat(dirPath); err != nil {
		fmt.Println("Cannot read the Path,", dirPath, "error message:", err)
		preCheckErrors = true
	} else if !info.IsDir() {
		fmt.Println("The Path", dirPath, "is not a directory!")
		preCheckErrors = true
	}

	// check if the DB report file has the extention and add if it doesn't have it
	if !strings.HasSuffix(DBfile, ".db") {
		DBfile += ".db"
	}
	// Check if the DB report file exists
	info, err := os.Stat(DBfile)
	if err == nil {
		if info.IsDir() {
			fmt.Println("The DBfile", DBfile, "cannot be a directory!")
			preCheckErrors = true
		} else {
			if !updateErrorOnly {
				fmt.Println("Looks like the DBfile", DBfile, "already exists.")
				fmt.Println("Either run the report to a new file or run with -updateErrorOnly=true options")
				preCheckErrors = true
			}
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if updateErrorOnly {
			fmt.Println("Looks like the DBfile", DBfile, "doesn't exists.")
			fmt.Println("Hence, -updateErrorOnly=false must be defined or this parameter must be omitted.")
			preCheckErrors = true
		}
	} else {
		fmt.Println("Error while checking", DBfile, "error message:", err)
		preCheckErrors = true
	}

	// exit if any error
	if preCheckErrors {
		os.Exit(0)
	}

	logFileName := strings.TrimSuffix(DBfile, ".db")               //log file name to store all the current logs
	timestamp := time.Now().Format("20060102_150405")              //Example format: 20240811_103045
	logFileName = fmt.Sprintf("%s_%s.log", logFileName, timestamp) //Append the current timestamp and .log suffix

	// Initialize loggers
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file %s: %v", logFileName, err)
		os.Exit(0)
	}
	defer logFile.Close()
	// Create a multi-writer to write to both file and console
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	// Create the logger that writes to both file and console
	infoMultiLogger = log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorMultiLogger = log.New(multiWriter, "ERR: ", log.Ldate|log.Ltime|log.Lshortfile)
	// Create the logger that writes to only file
	infoFileLogger = log.New(logFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

	infoMultiLogger.Println("Basic checks completed")
	infoMultiLogger.Println("Scanning", dirPath, "folder.")
	infoMultiLogger.Println("SQL report DB filename", DBfile)
	infoMultiLogger.Println("Scan only on the error folders?", updateErrorOnly)
	infoMultiLogger.Println("Is debugging enabled?", debug)
	infoMultiLogger.Println("Is UpdateWindowsFileOwner enabled?", updateWindowsFileOwner)
	fmt.Println("Logs will be saved to", logFileName, "file.")
	timestamp = time.Now().Format("20060102_150405") //reused the previous timestamp var as its not needed anymore
	infoMultiLogger.Println("Scan start time:", timestamp)

	FSdata := make(chan ObjectInfo, channelSize) //channel for new data
	infoMultiLogger.Printf("buffered channel of %d size created", channelSize)

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	if updateErrorOnly {
		infoMultiLogger.Println("Running scan on error folders only")
		var errorFolders []ErrorObjectInfo
		//block created to close the DB connection
		{
			// Open the database connection
			db, err := sql.Open("sqlite", DBfile)
			if err != nil {
				errorMultiLogger.Println(err)
				return
			}
			db.Exec("PRAGMA journal_mode=WAL;")
			defer db.Close()

			// Prepare the SQL query
			query := `SELECT Path, ObjectDepth FROM fileinfo WHERE ObjType = 'd' and hasError = '1';`

			// Execute the query
			rows, err := db.Query(query)
			if err != nil {
				errorMultiLogger.Println("failed to execute query:", query, "error is ", err)
				return
			}
			defer rows.Close()

			for rows.Next() {
				var errorFolder ErrorObjectInfo
				// Scan each row into the FileInfo struct
				err := rows.Scan(&errorFolder.Path, &errorFolder.ObjectDepth)
				if err != nil {
					errorMultiLogger.Println("Failed to scan a row:", err)
					return
				}
				// Add to the result slice
				errorFolders = append(errorFolders, errorFolder)
			}
		}
		infoMultiLogger.Println("Identified list of error folders are:")
		for _, error_folder := range errorFolders {
			infoMultiLogger.Printf("%v", error_folder)
			wg.Add(1)
			// atomic.AddInt32(&readFolderCounter, 1) // Increment the counter when a goroutine starts
			go readFolder(ctx, error_folder.Path, FSdata, error_folder.ObjectDepth, &wg)
		}
	} else {
		infoMultiLogger.Println("starting the 1st readFolder goroutine")
		wg.Add(1)
		// atomic.AddInt32(&readFolderCounter, 1) // Increment the counter when a goroutine starts
		go readFolder(ctx, dirPath, FSdata, 1, &wg)
	}

	infoMultiLogger.Println("Starting the writeMetaDataToSQliteDB goroutine")
	var wg2 sync.WaitGroup
	wg2.Add(1)
	go writeMetaDataToSQliteDB(FSdata, &wg2, cancel, DBfile)
	wg.Wait()
	close(FSdata)
	wg2.Wait()

	// postScanMetaDataUpdate()
	updateSizeLastWriteDate()
	timestamp = time.Now().Format("20060102_150405") //reused the previous timestamp var as its not needed anymore
	infoMultiLogger.Println("Scan end time:", timestamp)
	infoMultiLogger.Println("The End!")
}

// To read the folder contents
func readFolder(ctx context.Context, path string, FSdata chan<- ObjectInfo, depth int, wg *sync.WaitGroup) {
	defer wg.Done()
	// defer atomic.AddInt32(&readFolderCounter, -1) // Decrement the counter when done
	sem <- struct{}{}        // Acquire a token
	defer func() { <-sem }() // Release token after execution

	// infoFileLogger.Println("Number of readFolder routines running:", atomic.LoadInt32(&readFolderCounter))
	if debug {
		infoFileLogger.Printf("no of active/waiting goroutines: %d and pending data to be written to DB: %d", runtime.NumGoroutine(), len(FSdata))
	}

	if ctx.Err() != nil {
		errorMultiLogger.Printf("readFolder goroutine stopping at/for %s.\n", path)
		return
	}
	// build new ObjectInfo for the current folder
	currentFolderData := new(ObjectInfo)
	currentFolderData.ObjType = "d"
	currentFolderData.hasError = false
	currentFolderData.Path = path
	currentFolderData.ObjectDepth = depth
	currentFolderData.FileSize = 0
	currentFolderData.ThisFolderSize = 0

	// Get folder information
	info, err := os.Stat(path)
	if err != nil {
		currentFolderData.hasError = true
		currentFolderData.ErrorMessage = err.Error()
		errorMultiLogger.Printf("Failed to get directory info %s: %v", path, err)
	} else {
		//set the folder size which will just be the meta data size
		currentFolderData.ThisFolderSize = int(info.Size())

		ctime, atime, wtime, owner, err := getFileTimes(path)
		if err != nil {
			errorMultiLogger.Println(err)
		}
		currentFolderData.CreationTime = ctime
		currentFolderData.LastAccessTime = atime
		currentFolderData.LastWriteTime = wtime
		currentFolderData.Owner = owner

		// Read the directory contents
		entries, err := os.ReadDir(path)
		if err != nil {
			currentFolderData.hasError = true
			currentFolderData.ErrorMessage = err.Error()
			errorMultiLogger.Printf("Failed to read contents of directory %s: %v", path, err)
		} else {
			// Iterate over the directory entries
			var name string
			totalCurrentFolderSize := 0
			for _, entry := range entries {
				name = entry.Name()
				// Join the directory and file name
				fullPath := filepath.Join(path, name)
				if entry.IsDir() {
					wg.Add(1)
					// atomic.AddInt32(&readFolderCounter, 1) // Increment the counter when a goroutine starts
					go readFolder(ctx, fullPath, FSdata, depth+1, wg)
				} else {
					// build new ObjectInfo for the file
					newFileData := new(ObjectInfo)
					newFileData.ObjType = "f"
					newFileData.hasError = false
					newFileData.Path = fullPath
					newFileData.ObjectDepth = depth
					newFileData.FileSize = 0
					newFileData.ThisFolderSize = 0
					// Get file information
					// info, err := os.Stat(fullPath)
					info, err := entry.Info()
					if err != nil {
						newFileData.hasError = true
						newFileData.ErrorMessage = err.Error()
						errorMultiLogger.Printf("Failed to read file %s: %v", fullPath, err)
					} else {
						newFileData.FileSize = int(info.Size())
						totalCurrentFolderSize += newFileData.FileSize

						ctime, atime, wtime, owner, err := getFileTimes(fullPath)
						if err != nil {
							errorMultiLogger.Println(err)
						}
						newFileData.CreationTime = ctime
						newFileData.LastAccessTime = atime
						newFileData.LastWriteTime = wtime
						newFileData.Owner = owner
					}
					FSdata <- *newFileData
				}
			}
			currentFolderData.ThisFolderSize = totalCurrentFolderSize
		}
	}
	FSdata <- *currentFolderData
}

// To keep writing all the data in the channel to SQlite DB
func writeMetaDataToSQliteDB(FSdata <-chan ObjectInfo, wg2 *sync.WaitGroup, cancel context.CancelFunc, DBfile string) {
	defer wg2.Done()
	// Open a connection to the SQLite database
	db, err := sql.Open("sqlite", DBfile)
	if err != nil {
		errorMultiLogger.Printf("Unable to open SQlite connection in writeMetaDataToSQliteDB. Error msg: %v", err)
		errorMultiLogger.Printf("Sending cancellation signal from writeMetaDataToSQliteDB")
		cancel()
		return
	}
	defer db.Close()

	// Create a table if it doesn't already exist
	createTableSQL := `
    CREATE TABLE IF NOT EXISTS fileinfo (
        ObjType TEXT,
        Path TEXT PRIMARY KEY UNIQUE,
        ObjectDepth INTEGER,
		FileSize INTEGER,
        ThisFolderSize INTEGER,
        TotalCalFolderSize INTEGER,
        hasError BOOLEAN,
        ErrorMessage TEXT,
		Owner TEXT,
        CreationTime DATETIME,
        LastWriteTime DATETIME,
        CalLastWriteTime DATETIME,
        LastAccessTime DATETIME
    );`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		errorMultiLogger.Printf("Failed to create table: %v", err)
		errorMultiLogger.Println("Sending cancellation signal")
		cancel()
		return
	}

	placeholders := make([]string, 0, insertionBatchSizeSQL)
	values := make([]interface{}, 0, insertionBatchSizeSQL*11) // Each row has 11 values
	insertStmt := `INSERT INTO fileinfo (ObjType, Path, ObjectDepth, FileSize, ThisFolderSize, 
	hasError, ErrorMessage, Owner, CreationTime, LastWriteTime, LastAccessTime) VALUES `

	currentIteration := 0 //used to count the number of batch insertions done
	for data := range FSdata {
		// Add placeholders for each row
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		// Collect values for the placeholders
		values = append(values, data.ObjType, data.Path, data.ObjectDepth, data.FileSize,
			data.ThisFolderSize, data.hasError, data.ErrorMessage, data.Owner,
			data.CreationTime, data.LastWriteTime, data.LastAccessTime)

		// When we hit the batch size, execute the insert
		if len(placeholders) == insertionBatchSizeSQL {
			currentIteration++
			query := insertStmt + strings.Join(placeholders, ",")
			_, err := db.Exec(query, values...)
			if err != nil {
				errorMultiLogger.Printf("Failed to insert batch: %v", err)
			} else {
				if debug {
					infoFileLogger.Printf("Successfully inserted %d entries. Remaining entries are %d", currentIteration*insertionBatchSizeSQL, len(FSdata))
				}
			}
			// Reset placeholders and values for the next batch
			placeholders = placeholders[:0]
			values = values[:0]
		}
	}

	// Insert any remaining rows if there are fewer than batchSize
	if len(placeholders) > 0 {
		query := insertStmt + strings.Join(placeholders, ",")
		_, err := db.Exec(query, values...)
		if err != nil {
			errorMultiLogger.Printf("Failed to insert remaining batch: %v", err)
		} else {
			if debug {
				infoFileLogger.Printf("Successfully inserted %d entries. Remaining entries are %d", currentIteration*insertionBatchSizeSQL+len(placeholders), len(FSdata))
			}
		}
	}
	infoMultiLogger.Println("End of the DB insertion.")
}

// updateTotalCalSize updates TotalCalSize for each folder by summing its size and all its subfolders' sizes
func updateSizeLastWriteDate() {
	infoMultiLogger.Println("Starting the updateSizeLastWriteDate now")
	// Open the database connection
	db, err := sql.Open("sqlite", DBfile)
	if err != nil {
		errorMultiLogger.Println(err)
		return
	}
	db.Exec("PRAGMA journal_mode=WAL;")
	defer db.Close()

	// // Map to hold cumulative TotalCalSize for each folder
	// folderTotalCalSize := make(map[string]int)
	// // Map to hold cumulative CalLastWriteTime for each folder
	// folderLastWriteTimes := make(map[string]time.Time)

	// Map to hold cumulative TotalCalSize for each folder
	calculatedData := make(map[string]FolderInfoCal)

	// Prepare the SQL query
	query := `SELECT Path, ThisFolderSize,LastWriteTime FROM fileinfo WHERE ObjType = 'd';`
	// Execute the query
	rows, err := db.Query(query)
	if err != nil {
		errorMultiLogger.Println("failed to execute query:", err)
		return
	}
	defer rows.Close()

	// Loop through the result set
	for rows.Next() {
		var path string
		var size int
		var lastWriteTime time.Time

		// Scan the current row into variables
		if err := rows.Scan(&path, &size, &lastWriteTime); err != nil {
			errorMultiLogger.Println("failed to scan row:", err)
			return
		}
		// Process the data
		// folderTotalCalSize[path] += size
		// folderLastWriteTimes[path] = lastWriteTime

		// calculatedData[path] = FolderInfoCal{
		// 	TotalCalFolderSize: 1000,
		// 	CalLastWriteTime:   time.Now(),
		// }

		for {
			if folderInfo, exists := calculatedData[path]; exists {
				// If it exists, update the existing struct
				folderInfo.TotalCalFolderSize += size // Modify size
				if lastWriteTime.After(folderInfo.CalLastWriteTime) {
					folderInfo.CalLastWriteTime = lastWriteTime // Update last write time
				}
				calculatedData[path] = folderInfo // Save back updated struct
			} else {
				// If it does not exist, initialize and insert a new struct
				calculatedData[path] = FolderInfoCal{
					TotalCalFolderSize: size,          // Initial size
					CalLastWriteTime:   lastWriteTime, // Current time
				}
			}

			// Find the last separator (either '/' or '\')
			lastSeparator := strings.LastIndexAny(path, `\/`)
			if lastSeparator == -1 {
				break // No more separators, so we're at the root
			}

			if path[:lastSeparator] < dirPath {
				break // we have crossed the user provided directory
			} else if dirPath == path[:lastSeparator+1] {
				path = path[:lastSeparator+1] // Move up to the parent directory

			} else if dirPath == path[:lastSeparator] {
				path = path[:lastSeparator] // Move up to the parent directory
			} else {
				path = path[:lastSeparator] // Move up to the parent directory
			}

			// folderTotalCalSize[path] += size
			// if lastWriteTime.After(folderLastWriteTimes[path]) {
			// 	folderLastWriteTimes[path] = lastWriteTime
			// }
		}
	}

	// Now perform a batch update to the database for all folders
	tx, err := db.Begin() // Start a transaction for batch updating
	if err != nil {
		errorMultiLogger.Printf("failed to start transaction: %v", err)
		return
	}

	// Prepare the update statement
	updateStmt, err := tx.Prepare(`
		UPDATE fileinfo
		SET TotalCalFolderSize = ?, CalLastWriteTime = ?
		WHERE Path = ?;
	`)
	if err != nil {
		errorMultiLogger.Printf("failed to prepare update statement: %v", err)
		return
	}
	defer updateStmt.Close()

	// Batch update all folders
	for path, calData := range calculatedData {
		if _, err := updateStmt.Exec(calData.TotalCalFolderSize, calData.CalLastWriteTime, path); err != nil {
			tx.Rollback()
			errorMultiLogger.Printf("failed to update TotalCalFolderSize for %s: %v", path, err)
			return
		}
	}

	// Commit the transaction to apply the updates
	if err := tx.Commit(); err != nil {
		errorMultiLogger.Printf("failed to commit transaction: %v", err)
		return
	}

	infoMultiLogger.Println("End of updateSizeLastWriteDate")
}
