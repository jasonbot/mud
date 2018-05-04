package mud

// WorldBuilder handles map generation on top of the World, which is more a data store.
type WorldBuilder interface {
	StepInto(x1, y1, x2, y2 uint32)
	World() World
	GetUser(string) User
}

type worldBuilder struct {
	world World
}

func (builder *worldBuilder) StepInto(x1, y1, x2, y2 uint32) {
}

func (builder *worldBuilder) World() World {
	return builder.world
}

func (builder *worldBuilder) GetUser(username string) User {
	return builder.world.GetUser(username)
}

// NewWorldBuilder creates a new WorldBuilder to surround the World
func NewWorldBuilder(world World) WorldBuilder {
	return &worldBuilder{world: world}
}
