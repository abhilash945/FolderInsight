# FolderInsight
A Golang based project &amp; tool that scans all the subfolders, files and present the report in SQLite DB file.

Programming language: Golang  
External packages used:  
── golang.org/x/sys/unix       # for unix file times gather  
── modernc.org/sqlite          # Pure Go SQLite driver  



```
Project folder structure 
/FolderInsight/                         # Project root directory
│
├── /release/                           # Pre-compiled executables for different platforms
│   ├── FolderInsight-linux
│   ├── FolderInsight-windows.exe
│   └── FolderInsight-macos
├── /src/                               # Source code
│   ├── main.go                         # main application file
│   └── (other .go files)
│
├── LICENSE
├── README.md
```