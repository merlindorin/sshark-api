package globals

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricServer struct {
	Path string `env:"METRIC_PATH" default:"/metrics"`
}

func (o *MetricServer) Mount(g gin.IRouter) {
	g.GET(o.Path, gin.WrapH(promhttp.Handler()))
}
