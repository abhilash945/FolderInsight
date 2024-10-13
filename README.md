# FolderInsight
A Golang based project &amp; tool that scans all the subfolders, files and present the report in SQLite DB file.


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
```


```
Project folder structure:
/FolderInsight/                         # Project root directory
│
├── /release/                           # Pre-compiled executables for different platforms
│   ├── FolderInsight-linux
│   ├── FolderInsight.exe               # latest windows x64 release
│   └── FolderInsight-macos
├── main.go                             # main application file
├── (all the other .go files)
├── LICENSE
├── README.md
├── go.mod
├── go.sum
├── go.sum
```


Programming language: Golang  
External packages used:  
── golang.org/x/sys/unix       # for unix file times gather  
── modernc.org/sqlite          # Pure Go SQLite driver  


Release notes:

FolderInsight_v0.1.1.exe  
> '-BufferSize' and -SQLBatchSize' command line parameters are added  
> debug logs improved  

FolderInsight_v0.1.0.exe  
> Basic working model