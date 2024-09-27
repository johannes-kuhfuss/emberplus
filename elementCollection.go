/*
** Copyright (C) 2001-2024 Zabbix SIA
**
** This program is free software: you can redistribute it and/or modify it under the terms of
** the GNU Affero General Public License as published by the Free Software Foundation, version 3.
**
** This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;
** without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
** See the GNU Affero General Public License for more details.
**
** You should have received a copy of the GNU Affero General Public License along with this program.
** If not, see <https://www.gnu.org/licenses/>.
**/

package emberplus

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/s101"
)

// ElementKey used for element identification based on either element id or path.
type ElementKey struct {
	ID   string
	Path string
}

// ElementCollection contains one level of elements and their Ids as key.
type ElementCollection map[ElementKey]*Element

// Populate fills in collection with data from the decoder.
//
//nolint:gocyclo,cyclop
func (ec ElementCollection) Populate(data *asn1.Decoder) error {
	var end bool

	app0Codec, _, err := data.Read(asn1.RootElementCollectionTag, asn1.ApplicationByte)
	if err != nil {
		return fmt.Errorf("failed to read element root collection tag: %w", err)
	}

	app11Codec, _, err := app0Codec.Read(asn1.RootElementTag, asn1.ApplicationByte)
	if err != nil {
		return fmt.Errorf("failed to read element tag: %w", err)
	}

	for {
		var context0 *asn1.Decoder

		context0, _, err = app11Codec.Read(asn1.ContextZeroTag, asn1.ContextByte)
		if err != nil {
			return fmt.Errorf("failed to read top level context 0: %w", err)
		}

		var (
			decoder *asn1.Decoder
			el      *Element
		)

		el, decoder, err = getElement(context0)
		if err != nil {
			return fmt.Errorf("failed to read element: %w", err)
		}

		ec[ElementKey{ID: el.Identifier, Path: el.Path}] = el

		_, err = decoder.ReadEnd() // current context end
		if err != nil {
			return fmt.Errorf("failed to decode context end: %w", err)
		}

		_, err = decoder.ReadEnd() // current elements end
		if err != nil {
			return fmt.Errorf("failed to decode current sequence end: %w", err)
		}

		if decoder.Len() > 0 {
			app11Codec = asn1.NewDecoder(append(decoder.Bytes(), app11Codec.Bytes()...))
		}

		end, err = app11Codec.ReadEnd() // all  element end
		if err != nil {
			return fmt.Errorf("failed to decode element sequence end: %w", err)
		}

		if end {
			break
		}
	}

	end, err = app0Codec.ReadEnd() // end of the whole element
	if err != nil {
		return fmt.Errorf("failed to read sequence end of application 0 (the whole payload): %w", err)
	}

	if !end {
		return fmt.Errorf("main application decoder still has data remaining: %w", err)
	}

	return nil
}

// GetElementByPath returns element from collection with the provided path OID.
func (ec ElementCollection) GetElementByPath(currentPath string) (*Element, error) {
	for key, el := range ec {
		if key.Path == currentPath {
			return el, nil
		}

		for _, ch := range el.Children {
			childPath := fmt.Sprintf("%s.%s", key.Path, ch.Path)
			if childPath == currentPath {
				return ch, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to find element with path %q: %w", currentPath, ErrElementNotFound)
}

// GetElementByID returns element from collection with the provided identifier.
func (ec ElementCollection) GetElementByID(id string) (*Element, string, error) {
	for key, el := range ec {
		if key.ID == id {
			return el, key.Path, nil
		}

		for _, ch := range el.Children {
			if ch.Identifier == id {
				return ch, fmt.Sprintf("%s.%s", key.Path, ch.Path), nil
			}
		}
	}

	return nil, "", ErrElementNotFound
}

// MarshalJSON returns the collection with path(string) in key value instead of a structure for json marshaling.
func (ec ElementCollection) MarshalJSON() ([]byte, error) {
	out := make(map[string]any)

	for k, v := range ec {
		switch v.ElementType {
		case asn1.NodeType, asn1.QualifiedNodeType:
			out[k.Path] = node{
				Path:        v.Path,
				ElementType: v.ElementType,
				Identifier:  v.Identifier,
				Description: v.Description,
				Children:    v.Children,
				IsOnline:    v.IsOnline,
				IsRoot:      v.IsRoot,
			}
		case asn1.ParameterType, asn1.QualifiedParameterType:
			out[k.Path] = parameter{
				Path:        v.Path,
				ElementType: v.ElementType,
				Children:    v.Children,
				Identifier:  v.Identifier,
				Description: v.Description,
				Value:       v.Value,
				Minimum:     v.Minimum,
				Maximum:     v.Maximum,
				Access:      v.Access,
				Format:      v.Format,
				Enumeration: v.Enumeration,
				Factor:      v.Factor,
				IsOnline:    v.IsOnline,
				Default:     v.Default,
				ValueType:   v.ValueType,
			}
		case asn1.FunctionType:
			out[k.Path] = function{
				Path:        v.Path,
				ElementType: v.ElementType,
				Identifier:  v.Identifier,
				Description: v.Description,
			}
		default:
			return nil, errors.New("failed unknown element type")
		}
	}

	bytes, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed native marshal: %w", err)
	}

	return bytes, nil
}

// NewElementConnection creates a empty element collection.
func NewElementConnection() ElementCollection {
	return make(ElementCollection)
}

// GetRootRequest returns a S101 request packet with an encoded request for root collection.
func GetRootRequest() ([]byte, error) {
	encoder := asn1.NewEncoder()

	err := encoder.WriteRootTreeRequest()
	if err != nil {
		return nil, fmt.Errorf("failed to write root command request: %w", err)
	}

	return s101.Encode(encoder.GetData(), s101.FirstMultiPacket), nil
}

// GetRequestByType returns S101 packet with an encoded request for element with the provided type and path.
func GetRequestByType(et ElementType, path string) ([]byte, error) {
	encoder := asn1.NewEncoder()

	parsed, err := parsePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	err = encoder.WriteRequest(parsed, string(et), asn1.EmberGetDirCommand)
	if err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	return s101.Encode(encoder.GetData(), s101.FirstMultiPacket), nil
}
