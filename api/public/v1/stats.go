package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
)

func Stats(c *gin.Context, logger *zap.Logger, repo sources.Repository) {
	ctx := c.Request.Context()

	stats, err := repo.GetStats(ctx)
	if err != nil {
		logger.Error("failed to get stats", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	facets := make(map[string][]public.Facet)
	for field, domainFacets := range stats.Facets {
		apiFacets := make([]public.Facet, len(domainFacets))
		for i, f := range domainFacets {
			data := make([]public.FacetValue, len(f.Data))
			for j, v := range f.Data {
				data[j] = public.FacetValue{Value: v.Value, Count: v.Count}
			}
			apiFacets[i] = public.Facet{Type: public.FacetType(f.Type), Data: data}
		}
		facets[field] = apiFacets
	}

	c.JSON(http.StatusOK, public.Statistics{Facets: facets})
}
