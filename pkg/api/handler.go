package api

import "github.com/gin-gonic/gin"

// Response is
type Response struct {
	Code    int
	Message interface{}
}

type Handler interface {
	ExecSearch(c *gin.Context) (*Response, Error)
	GetSearchLogs(c *gin.Context) (*Response, Error)
	GetSearchTimeSeries(c *gin.Context) (*Response, Error)
}

type MinervaHandler struct {
	DatabaseName     string
	IndexTableName   string
	MessageTableName string
	OutputPath       string
	Region           string
}

// Handler is handler interface
func sendResponse(c *gin.Context, resp *Response, err Error) {
	if err != nil {
		c.JSON(err.Code(), gin.H{"message": err.Message()})
	} else {
		c.JSON(resp.Code, resp.Message)
	}
}
