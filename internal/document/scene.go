package document

import "nimbleterminal.dev/nt3d"

// SetScene replaces the viewer's mesh and explicitly normalizes it to the
// two-unit world used by Gloss's named views and camera-distance options.
// Scene takes its own geometry snapshot, so recoloring the document cannot
// mutate a frame already being rendered.
func (m *Mesh) SetScene(scene *nt3d.Scene) error {
	transform, _, scale := m.geometry.Bounds.Normalization()
	g := m.geometry
	if m.pointRadius > 0 {
		// Point radii in world units are not scaled by the node's transform:
		// they are given in the normalized world.
		g.Points = append([]nt3d.Point(nil), g.Points...)
		for i := range g.Points {
			g.Points[i].Radius = m.pointRadius * scale
		}
		g.PointStyle.Units = nt3d.WorldUnits
	}
	node, err := scene.SetGeometry("mesh", g)
	if err != nil {
		return err
	}
	return scene.SetTransform(node, transform)
}

func (m *Mesh) Scene() (*nt3d.Scene, error) {
	scene := nt3d.NewScene()
	if err := m.SetScene(scene); err != nil {
		return nil, err
	}
	return scene, nil
}
