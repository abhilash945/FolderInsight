# FolderInsight
A Golang based project &amp; tool that scans all the files & folders and generate a meta data report in SQLite .db file.

This is a simple tool that take the folder path (local or on network) and a report/output file name as parameters and generate a report in a .DB file. This will gather the properties of all the folders and files like name, depth, size, error, creation time, last write time, last access time, Total folder size(including the sub folders & files), calculated last write time(derived from the inner files & folders) and optionally the owner name.

All this data gathering happens at a faster speed using the options like goroutines, channels and certain in memory-based operations.

```
Tool usage syntax:
PS C:\FolderInsight> .\FolderInsight.exe -help
Usage of C:\FolderInsight\FolderInsight.exe:
  -BufferSize int
        meta data buffer size (optional) (default 100000)
  -DBfile string
        Result report DB file (mandatory)
  -Path string
        Folder to scan (mandatory)
  -SQLBatchSize int
        DB batch size for buffered insertions (optional) (default 200)
  -UpdateErrorOnly
        Run scan only on failed directories (optional, default is false)
  -debug
        Enable debug logging (optional, default is false)
PS C:\FolderInsight>
Example usage:
.\FolderInsight.exe -DBfile=temp -Path="C:\Temp"
.\FolderInsight.exe -DBfile=temp -Path="C:\Temp" -UpdateWindowsFileOwner=true
.\FolderInsight.exe -DBfile=temp -Path="C:\Temp" -UpdateWindowsFileOwner=true -debug=true
.\FolderInsight.exe -DBfile=temp -Path="C:\Temp" -debug=true
.\FolderInsight.exe -DBfile=temp -Path="C:\Temp" -UpdateErrorOnly=true
.\FolderInsight.exe -DBfile=temp -Path="C:\Temp" -UpdateErrorOnly=true -debug=true
```



```
Project folder structure:
/FolderInsight/                         # Project root directory
│
├── /release/                           # pre-compiled executables for different platforms
│   ├── FolderInsight-linux
│   ├── FolderInsight.exe               # latest windows x64 release
│   └── FolderInsight-macos
├── main.go                             # main application file
├── (all the other .go files)
├── LICENSE
├── README.md
├── go.mod
├── go.sum
```

General notes:  
Programming language: Golang  
Supported Windows Operating Systems:  
Windows 7 and later  
Windows Server 2012 and later  


External packages used:  
── golang.org/x/sys/unix       # for unix file times gather  
── modernc.org/sqlite          # Pure Go SQLite driver  


Release notes:  
FolderInsight_v0.1.1  
. Renamed few existing DB columns.  
. Added new column TotalCalFolderSize & CalLastWriteTime in the DB report.  
. Added, in-memeory processing of TotalCalFolderSize & CalLastWriteTime values.  
. Added optional '-updateWindowsFileOwner' field to gather file/folder owner name optionally.  

FolderInsight_v0.1.0  
. Basic working model  
. '-BufferSize' and -SQLBatchSize' command line parameters are added  
. debug logs improved  


Contact me:  
abhilash945@gmail.com