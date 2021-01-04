## gURL - generated URL
A simple Golang URL shortener thrown together then expanded upon for a friend.

### Endpoints

 - /c/{url} - Provides a JSON response with the relevent information and shortened generated URL (gurl)
 - /b/{key} - "B" for bounce, this endpoint takes a key and preforms the redirect.
 - /d/{key} - Delete a key from our database before its expire date.
### Command Line Arguments

    Usage of gurl:
      -b string
        	A simple bindhost string, eg: ":9999" or "127.0.0.1" (default ":9999")
      -c string
        	set the time delta for cache expiry (default "24h")
      -d uint
        	set how often to insert a dash (default 5)
      -l uint
        	set the generated uri string length (default 10)


<sup><sup><sub>For Kozaid, Lord of SyntaxErrors</sub></sub></sub>
