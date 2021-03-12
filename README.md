## gURL - generated URL
A simple Golang URL shortener thrown together then expanded upon for a friend.

### Endpoints
 - POST   /c/{url} - Provides a JSON response with the relevent information and shortened generated URL (gurl)
 - GET    /b/{key} - "b" for bounce, this endpoint takes a key and preforms the redirect.
 - DELETE /d/{key}/{token} - Remove a key from our database before its expire date using its one-time-token.
### Command Line Arguments

    Usage of gurl:
        -acao string
    	    Set the Access-Control-Allow-Origin header (default "*")
        -addr string
    	    A simple bindhost string, eg: ":9999" or "127.0.0.1" (default ":9999")
        -dir string
    	    set the directory for web/html files served at webroot (default "./static")
        -exp string
    	    set the time delta for cache expiry (default "24h")
        -len uint
    	    set the generated uri string length (default 10)
        -log string
    	    set the alert/warn level of the logging. Info, Warn, Error, Fatal, Panic (default "Info")
        -sep uint
    	    set how often to insert a dash (default 5)


<sup><sup><sub>For Kozaid, Lord of SyntaxErrors</sub></sub></sub>
