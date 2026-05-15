package logs

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func TailHandler(c *gin.Context) {

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, gin.H{"error": "stream not supported"})
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Fprintf(c.Writer, "data: log line %d\n\n", i)
		flusher.Flush()
		time.Sleep(1 * time.Second)
	}
}