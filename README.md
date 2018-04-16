## how to run ##
```
go run main.go -baseURL http://127.0.0.1:8000/ -outDir /tmp -stateDir /tmp
```

#### assumptions ####
* assuming urls are relatively small so we can store them in memory
* if input url = "http://google.com/one" then
"http://google.com/onetwothree" should not be processed    
    
    
#### text file detection ####
We detect text files by extension.
For now urls ending with '.txt', '.md', '.css', '.csv', '.json', '.xml'
are considered text files. Gzipped text files are not considered
text files.
Also, we download URLs from other domains as well 
(maybe they store files in some cloud or something).

Other ways to detect if file is text file:
* do HEAD request and check 'content-type' response header
* start to download and check first bytes of response

#### pages detection ####
We try to download every non-text-url and check 
if response is of content type 'text/html'.
If it is so we download it. 