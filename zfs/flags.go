package zfs

// Differentiate truly readonly status flags from onlyAtCreation flags
var statusProperties = map[string]struct{}{
	"type":                 {},
	"creation":             {},
	"used":                 {},
	"available":            {},
	"referenced":           {},
	"compressratio":        {},
	"mounted":              {},
	"version":              {},
	"defer_destroy":        {},
	"userrefs":             {},
	"usedbysnapshots":      {},
	"usedbydataset":        {},
	"usedbychildren":       {},
	"usedbyrefreservation": {},
	"refcompressratio":     {},
	"written":              {},
	"clones":               {},
	"logicalused":          {},
	"logicalreferenced":    {},
	"encryptionroot":       {},
	"keystatus":            {},
}
