# buildathon-mech

Go bot for the Builders Buildathon **Mech Royale** track (`mechfight/1`).

- `main.go` – stdin/stdout JSON-lines loop (answers only the newest observation)
- `proto.go` – message types
- `world.go` – map raster (terrain + prisms), line-of-sight, 2 m nav grid, A*
- `brain.go` – zone, targeting, lead aim, laser/lightning/mortar use, dodging, loot, cover, upgrades

Build: `docker build -t mech-bot .` (static binary in a `scratch` image; map read from `/arena`).
