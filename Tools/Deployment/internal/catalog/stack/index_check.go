package stack

import (
	"fmt"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// LayerIndexReport is the workflow-index check of one layer.
type LayerIndexReport struct {
	Layer  domain.CatalogLayer
	Report catalog.WorkflowIndexReport // findings carry this layer's paths
}

// CheckIndex runs catalog.CheckWorkflowIndex once per layer, highest first. An error names the
// layer it occurred in.
func CheckIndex(c Catalog) ([]LayerIndexReport, error) {
	layers := c.Layers()
	reports := make([]LayerIndexReport, 0, len(layers))
	for _, l := range layers {
		rep, err := catalog.CheckWorkflowIndex(l.Root)
		if err != nil {
			return nil, fmt.Errorf("catalog layer %s (%s): %w", l.Identity.ID, l.Root, err)
		}
		reports = append(reports, LayerIndexReport{Layer: l, Report: rep})
	}
	return reports, nil
}
