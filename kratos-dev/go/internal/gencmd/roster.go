package gencmd

// CanonicalOrder is the hand-curated god display order used in SKILL.md's
// frontmatter description god-name list. It is preserved verbatim (not
// alphabetically derived) to avoid needless first-run diff noise; membership
// against the live agent roster is still validated (see validateRoster).
// Adding a god requires appending its display name here — see
// kratos-dev/codegen/README.md.
var CanonicalOrder = []string{
	"Athena", "Ares", "Metis", "Apollo", "Artemis", "Hermes", "Hephaestus",
	"Daedalus", "Clio", "Mimir", "Hades", "Odysseus", "Prometheus", "Themis",
	"Nemesis", "Cassandra", "Hera", "Ananke", "Iris",
}

// QuickGodOrder is the hand-curated order of quick-mode gods (quick_route:
// true) shown in SKILL.md's Activation bullet. Membership is validated
// against each agent's quick_route field.
var QuickGodOrder = []string{
	"Artemis", "Ares", "Hermes", "Metis", "Daedalus", "Hades", "Odysseus",
}

// OwnCommandGodOrder is the hand-curated order of non-quick-mode gods shown in
// SKILL.md's Activation bullet. Membership is validated against each agent's
// quick_route field.
var OwnCommandGodOrder = []string{
	"Athena", "Apollo", "Cassandra", "Clio", "Mimir", "Nemesis", "Hephaestus",
	"Hera", "Themis", "Prometheus", "Ananke", "Iris",
}
