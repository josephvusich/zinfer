package zfs

// Differentiate truly readonly status flags from onlyAtCreation flags
var statusProperties = map[string]struct{}{
	"type":                 {},
	"creation":             {},
	"used":                 {},
	"available":            {},
	"referenced":           {},
	"rekeydate":            {},
	"compressratio":        {},
	"mounted":              {},
	"origin":               {},
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

	"size":          {},
	"capacity":      {},
	"health":        {},
	"dedupratio":    {},
	"free":          {},
	"allocated":     {},
	"expandsize":    {},
	"freeing":       {},
	"fragmentation": {},
	"leaked":        {},
	"checkpoint":    {},
}

// Properties that do not appear readonly, but should not be included in output
var ignoreProperties = map[string]struct{}{
	"readonly": {}, // Can only be set during import
}

var encryptionRoot = "encryptionroot"

// Properties that inherit from encryptionroot rather than parent
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
