/*
** Copyright (C) 2001-2024 Zabbix SIA
** Adaptations (C) 2024 JKU
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

package ember

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

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

// Populate fills the collection while preserving the legacy value and element representations.
func (ec ElementCollection) Populate(data *asn1.Decoder) error {
	return ec.populateLegacy(data)
}

// PopulateGlow250 fills the collection using the complete Glow 2.50 model.
func (ec ElementCollection) PopulateGlow250(data *asn1.Decoder) error {
	message, err := DecodeRoot(data.Bytes())
	if err != nil {
		return fmt.Errorf("failed to decode Glow root: %w", err)
	}
	if message.Elements == nil {
		return errors.New("Glow root does not contain elements")
	}
	for key, element := range message.Elements {
		ec[key] = element
	}
	return nil
}

// populateLegacy is retained for source compatibility with the original model.
//
//nolint:gocyclo,cyclop
func (ec ElementCollection) populateLegacy(data *asn1.Decoder) error {
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

		if child, ok := findElementByPath(el.Children, key.Path, currentPath); ok {
			return child, nil
		}
	}

	return nil, fmt.Errorf("failed to find element with path %q: %w", currentPath, ErrElementNotFound)
}

// GetElementByID returns element from collection with the provided identifier.
func (ec ElementCollection) GetElementByID(id string) (*Element, string, error) {
	matches := ec.GetElementsByID(id)
	if len(matches) > 0 {
		return matches[0].Element, matches[0].Path, nil
	}

	return nil, "", ErrElementNotFound
}

// ElementMatch is one scoped identifier lookup result.
type ElementMatch struct {
	Element *Element
	Path    string
}

// GetElementsByID returns every element with id in deterministic path order.
// Ember identifiers are only expected to be unique among siblings.
func (ec ElementCollection) GetElementsByID(id string) []ElementMatch {
	var matches []ElementMatch
	for key, el := range ec {
		if key.ID == id {
			matches = append(matches, ElementMatch{Element: el, Path: key.Path})
		}
		collectElementsByID(&matches, el.Children, key.Path, id)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Path < matches[j].Path })
	return matches
}

func collectElementsByID(matches *[]ElementMatch, children []*Element, parentPath, id string) {
	for _, child := range children {
		childPath := joinElementPath(parentPath, child.Path)
		if child.Identifier == id {
			*matches = append(*matches, ElementMatch{Element: child, Path: childPath})
		}
		collectElementsByID(matches, child.Children, childPath, id)
	}
}

func findElementByPath(children []*Element, parentPath, currentPath string) (*Element, bool) {
	for _, child := range children {
		childPath := joinElementPath(parentPath, child.Path)
		if childPath == currentPath {
			return child, true
		}

		if found, ok := findElementByPath(child.Children, childPath, currentPath); ok {
			return found, true
		}
	}

	return nil, false
}

func findElementByID(children []*Element, parentPath, id string) (*Element, string, bool) {
	for _, child := range children {
		childPath := joinElementPath(parentPath, child.Path)
		if child.Identifier == id {
			return child, childPath, true
		}

		if found, foundPath, ok := findElementByID(child.Children, childPath, id); ok {
			return found, foundPath, true
		}
	}

	return nil, "", false
}

func joinElementPath(parent, child string) string {
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}

// MarshalJSON returns the collection with path(string) in key value instead of a structure for json marshaling.
func (ec ElementCollection) MarshalJSON() ([]byte, error) {
	out := make(map[string]any)

	for k, v := range ec {
		switch v.ElementType {
		case asn1.NodeType, asn1.QualifiedNodeType:
			out[k.Path] = node{
				Path:              v.Path,
				ElementType:       v.ElementType,
				Identifier:        v.Identifier,
				Description:       v.Description,
				Children:          v.Children,
				IsOnline:          v.IsOnline,
				IsRoot:            v.IsRoot,
				SchemaIdentifiers: v.SchemaIdentifiers,
				TemplateReference: v.TemplateReference,
			}
		case asn1.ParameterType, asn1.QualifiedParameterType:
			out[k.Path] = parameter{
				Path:              v.Path,
				ElementType:       v.ElementType,
				Children:          v.Children,
				Identifier:        v.Identifier,
				Description:       v.Description,
				Value:             v.Value,
				HasValue:          v.HasValue,
				Minimum:           v.Minimum,
				Maximum:           v.Maximum,
				Access:            v.Access,
				Format:            v.Format,
				Enumeration:       v.Enumeration,
				Factor:            v.Factor,
				IsOnline:          v.IsOnline,
				Default:           v.Default,
				HasDefault:        v.HasDefault,
				ValueType:         v.ValueType,
				Formula:           v.Formula,
				Step:              v.Step,
				StreamIdentifier:  v.StreamIdentifier,
				EnumMap:           v.EnumMap,
				StreamDescriptor:  v.StreamDescriptor,
				SchemaIdentifiers: v.SchemaIdentifiers,
				TemplateReference: v.TemplateReference,
			}
		case asn1.FunctionType, QualifiedFunctionElement:
			out[k.Path] = function{
				Path:        v.Path,
				ElementType: v.ElementType,
				Identifier:  v.Identifier,
				Description: v.Description,
				Signature:   v.Function,
			}
		case MatrixElement, QualifiedMatrixElement, TemplateElement, QualifiedTemplateElement, CommandElement:
			out[k.Path] = v
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

// NewElementConnection creates an empty element collection.
func NewElementConnection() ElementCollection {
	return make(ElementCollection)
}

// NewElementCollection creates an empty element collection.
func NewElementCollection() ElementCollection {
	return make(ElementCollection)
}

// GetRootRequest returns a S101 request packet with an encoded request for root collection.
func GetRootRequest() ([]byte, error) {
	glow, err := EncodeGetDirectory(QualifiedNodeElement, "", -1)
	if err != nil {
		return nil, fmt.Errorf("failed to encode root command request: %w", err)
	}
	return s101.Encode(glow, s101.SinglePacket), nil
}

// GetRequestByType returns S101 packet with an encoded request for element with the provided type and path.
func GetRequestByType(et ElementType, path string) ([]byte, error) {
	glow, err := EncodeGetDirectory(et, path, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}
	return s101.Encode(glow, s101.SinglePacket), nil
}
