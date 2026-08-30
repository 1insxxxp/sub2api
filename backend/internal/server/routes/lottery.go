package routes

import (
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
)

func registerLotteryRoutes(admin *gin.RouterGroup, h *adminhandler.LotteryHandler) {
	lottery := admin.Group("/lottery")
	{
		lottery.GET("/config", h.GetConfig)
		lottery.PUT("/activity", h.SaveActivity)
		lottery.POST("/prizes", h.CreatePrize)
		lottery.PUT("/prizes/:id", h.UpdatePrize)
		lottery.DELETE("/prizes/:id", h.DeletePrize)
		lottery.GET("/prizes/:id/items", h.ListItems)
		lottery.POST("/prizes/:id/items", h.AppendItems)
		lottery.DELETE("/prizes/:id/items", h.DeleteItems)
	}
}
