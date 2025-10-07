package uploads

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(r *gin.Engine, db *gorm.DB) {
	r.Static("/uploads", "./uploads")
}
