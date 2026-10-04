package intelligence

// Analysis is the lightweight repository analysis used by interactive flows.
// Detailed repository detection uses ProjectModel; this type remains only for
// the interactive confirmation UI until that flow is migrated separately.
type Analysis struct {
	Root string
	Name string
}
