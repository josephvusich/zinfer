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

var encryptionRoot = "encryptionroot"

// Properties inherited from encryptionroot
// Note that encryptionLocalProperties overlaps with this set
var encryptionInheritedProperties = map[string]struct{}{
	"encryptionroot": {},
	"encryption":     {},
	"keylocation":    {}, // This one appears to be "none, local" on child datasets, but we treat it like the others
	"keyformat":      {},
	"pbkdf2iters":    {},
	"keystatus":      {},
}

// Properties that may differ from encryptionroot
var encryptionLocalProperties = map[string]struct{}{
	"encryption": {},
}
