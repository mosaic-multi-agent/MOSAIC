package deploy

// content_cache.go keeps the content a run renders for a created item stable across the run.
// The workspace probe writes the first renderable item's real target file before the item
// itself is executed; rendering a created item again afterwards would see its own just-written
// file as prior deployed bytes, which turns a create into a create over existing bytes. A
// created item has no prior file, so the first render, made before anything is written, is the
// right one for the rest of the run.

import "mosaic-deploy/internal/domain"

// cachingContent wraps content so that the first successful render of each created item is
// returned for every later request of that path. Items of any other action, and failed
// renders, are never cached.
func cachingContent(content func(domain.PlanItem) ([]byte, error)) func(domain.PlanItem) ([]byte, error) {
	if content == nil {
		return nil
	}
	rendered := map[string][]byte{}
	return func(item domain.PlanItem) ([]byte, error) {
		if item.Action != domain.ActionCreate {
			return content(item)
		}
		if b, ok := rendered[item.TargetPath]; ok {
			return b, nil
		}
		b, err := content(item)
		if err != nil {
			return nil, err
		}
		rendered[item.TargetPath] = b
		return b, nil
	}
}
