package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/Iknite-Space/sqlc-example-api/db/repo"
	"github.com/Iknite-Space/sqlc-example-api/helper"
	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	querier repo.Querier
}

func NewMessageHandler(querier repo.Querier) *MessageHandler {
	return &MessageHandler{
		querier: querier,
	}
}

func (h *MessageHandler) WireHttpHandler() http.Handler {

	r := gin.Default()
	r.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.String(http.StatusInternalServerError, "Internal Server Error: panic")
		c.AbortWithStatus(http.StatusInternalServerError)
	})) //prevents the server from crashing if an error occurs in any route

	r.POST("/thread", h.handleCreateThread)
	r.POST("/message", h.handleCreateMessage)
	r.GET("/message/:id", h.handleGetMessage)
	// r.GET("/thread/:threadId/messages", h.handleGetThreadMessages)
	r.DELETE("/message/:id", h.handleDeleteMessageById)
	r.DELETE("/thread/:threadId/messages", h.handleDeleteMessageByThreadId)
	r.PATCH("/message", h.handleUpdateMessage)
	r.POST("/order", h.handleCreateOrder)
	r.GET("/thread/:threadId/messages", h.handleGetPaginatedThreadMessages)

	return r
}

type CreateThreadParams struct {
	Title string `json:"title"`
}

func (h *MessageHandler) handleCreateThread(c *gin.Context) {
	var req CreateThreadParams
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	thread, err := h.querier.CreateThread(c, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, thread)
}

func (h *MessageHandler) handleCreateMessage(c *gin.Context) {
	var req repo.CreateMessageParams
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//first check whether the thread exist
	_, err := h.querier.GetThreadById(c, req.ThreadID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Thread not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
		return
	}

	//now we proceed to create the message
	message, err := h.querier.CreateMessage(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, message)
}

func (h *MessageHandler) handleGetMessage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	message, err := h.querier.GetMessageByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, message)
}

// func (h *MessageHandler) handleGetThreadMessages(c *gin.Context) {
// 	id := c.Param("threadId")
// 	intVal, err := helper.GetParamAsInt32(id)

// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, err.Error())
// 		return
// 	}

// 	messages, err := h.querier.GetMessagesByThread(c, intVal)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	if len(messages) == 0 {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "No messages found for this thread"})
// 	}

// 	c.JSON(http.StatusOK, messages)
// }

func (h *MessageHandler) handleUpdateMessage(c *gin.Context) {
	var req repo.UpdateMessageParams
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.querier.UpdateMessage(c, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": "Message updated successfully"})
}

func (h *MessageHandler) handleDeleteMessageById(c *gin.Context) {
	id := c.Param("id")

	_, err := h.querier.DeleteMessageById(c, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted successfully"})

}

func (h *MessageHandler) handleDeleteMessageByThreadId(c *gin.Context) {
	id := c.Param("threadId")

	intId, err := helper.GetParamAsInt32(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	_, err = h.querier.DeleteMessageByThreadId(c, intId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted successfully"})

}

func (h *MessageHandler) handleCreateOrder(c *gin.Context) {
	var req repo.CreateOrderParams

	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	order, err := h.querier.CreateOrder(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}
	token := os.Getenv("AUTH_TOKEN")
	reference, err := requestPayment(req.PhoneNumber, fmt.Sprintf("%d", req.Amount), "Order Payment", token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order":     order,
		"reference": reference,
		"message":   "Order created and payment request initiated.",
	})
}

// paginated thread messages
func (h *MessageHandler) handleGetPaginatedThreadMessages(c *gin.Context) {
	id := c.Param("threadId")

	threadId, err := helper.GetParamAsInt32(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	//parse pagination query params
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit <= 0 {
		limit = 10
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	messages, err := h.querier.GetMessageByThreadPaginated(c, repo.GetMessageByThreadPaginatedParams{
		ThreadID: threadId,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch messages"})
	}

	if len(messages) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "No messages found"})
		return
	}

	c.JSON(http.StatusOK, messages)
}
