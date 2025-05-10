package anki

// Export represents the structure of an Anki export file
type Export struct {
	Type        string   `json:"__type__,omitempty"`
	MediaFiles  []string `json:"media_files,omitempty"`
	Notes       []Note   `json:"notes,omitempty"`
	NoteModels  []Model  `json:"note_models,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"desc,omitempty"`
}

// Note represents a note in the Anki deck
type Note struct {
	Type      string   `json:"__type__,omitempty"`
	Fields    []string `json:"fields,omitempty"`
	GUID      string   `json:"guid,omitempty"`
	ModelUUID string   `json:"note_model_uuid,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// Model represents a note model in the Anki deck
type Model struct {
	Type   string  `json:"__type__,omitempty"`
	UUID   string  `json:"crowdanki_uuid,omitempty"`
	Name   string  `json:"name,omitempty"`
	CSS    string  `json:"css,omitempty"`
	Fields []Field `json:"flds,omitempty"`
}

// Field represents a field in an Anki note model
type Field struct {
	Name string `json:"name,omitempty"`
	Ord  int    `json:"ord,omitempty"`
}

// MediaFile represents a media file from the Anki export
type MediaFile struct {
	FileName    string
	ContentType string
	FilePath    string
}

// ImportResult contains the results of an Anki import operation
type ImportResult struct {
	DeckName      string
	CardsAdded    int
	MediaUploaded int
	Errors        []string
}
