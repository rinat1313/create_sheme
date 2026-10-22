package domain

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type FileSolution struct {
	NameFile string
	Discript string
}

type CommandStruct struct {
	Id             string
	NameTemplate   string
	AfterHeaderTag string
}

type AutomationStruct struct {
	Id             string
	NameTemplate   string
	AfterHeaderTag string
}

type AnchorPoint struct {
	X string `xml:"X,attr"`
	Y string `xml:"Y,attr"`
}

type Automation struct {
	Description        string               `xml:"Description,attr"`
	ExternalCode       string               `xml:"ExternalCode,attr"`
	Id                 string               `xml:"Id,attr"`
	IdFromSource       string               `xml:"IdFromSource,attr"`
	Name               string               `xml:"Name,attr"`
	Tag                string               `xml:"Tag,attr"`
	Target             string               `xml:"Target,attr"`
	Template           string               `xml:"Template,attr"`
	TechnologyProperty []TechnologyProperty `xml:"TechnologyProperty"`
}

type Automations struct {
	Automation []Automation `xml:"Automation"`
}

type ControlCommand struct {
	Description        string               `xml:"Description,attr"`
	ExternalCode       string               `xml:"ExternalCode,attr"`
	Id                 string               `xml:"Id,attr"`
	IdFromSource       string               `xml:"IdFromSource,attr"`
	Name               string               `xml:"Name,attr"`
	Tag                string               `xml:"Tag,attr"`
	Target             string               `xml:"Target,attr"`
	Template           string               `xml:"Template,attr"`
	TechnologyProperty []TechnologyProperty `xml:"TechnologyProperty"`
}

type ControlCommands struct {
	ControlCommand []ControlCommand `xml:"ControlCommand"`
}

type GraphicLabel struct {
	AutoLocation         string      `xml:"AutoLocation,attr"`
	Description          string      `xml:"Description,attr"`
	ExternalCode         string      `xml:"ExternalCode,attr"`
	Id                   string      `xml:"Id,attr"`
	IdFromSource         string      `xml:"IdFromSource,attr"`
	Name                 string      `xml:"Name,attr"`
	Tag                  string      `xml:"Tag,attr"`
	VisualDeclarationKey string      `xml:"VisualDeclarationKey,attr"`
	AnchorPoint          AnchorPoint `xml:"AnchorPoint"`
}

type GraphicLabels struct {
	GraphicLabel []GraphicLabel `xml:"GraphicLabel"`
}

type GraphicObject struct {
	Angle                 string             `xml:"Angle,attr"`
	Description           string             `xml:"Description,attr"`
	ExternalCode          string             `xml:"ExternalCode,attr"`
	Id                    string             `xml:"Id,attr"`
	IdFromSource          string             `xml:"IdFromSource,attr"`
	IsHorizontalReflected string             `xml:"IsHorizontalReflected,attr"`
	IsLocked              string             `xml:"IsLocked,attr"`
	IsReversed            string             `xml:"IsReversed,attr"`
	IsVerticalReflected   string             `xml:"IsVerticalReflected,attr"`
	IsVisibled            string             `xml:"IsVisibled,attr"`
	LinkedGraphicScheme   string             `xml:"LinkedGraphicScheme,attr"`
	Name                  string             `xml:"Name,attr"`
	Tag                   string             `xml:"Tag,attr"`
	AnchorPoint           []AnchorPoint      `xml:"AnchorPoint"`
	GraphicLabels         []GraphicLabels    `xml:"GraphicLabels"`
	Points                []Points           `xml:"Points"`
	TechnologyObject      []TechnologyObject `xml:"TechnologyObject"`
}

type GraphicScheme struct {
	Description   string          `xml:"Description,attr"`
	ExternalCode  string          `xml:"ExternalCode,attr"`
	Id            string          `xml:"Id,attr"`
	IdFromSource  string          `xml:"IdFromSource,attr"`
	Name          string          `xml:"Name,attr"`
	Tag           string          `xml:"Tag,attr"`
	Template      string          `xml:"Template,attr"`
	GraphicObject []GraphicObject `xml:"GraphicObject"`
}

type ObjectReference struct {
	Description  string `xml:"Description,attr"`
	ExternalCode string `xml:"ExternalCode,attr"`
	Id           string `xml:"Id,attr"`
	IdFromSource string `xml:"IdFromSource,attr"`
	Name         string `xml:"Name,attr"`
	Tag          string `xml:"Tag,attr"`
	Target       string `xml:"Target,attr"`
}

type ObjectReferences struct {
	ObjectReference []ObjectReference `xml:"ObjectReference"`
}

type Point struct {
	X string `xml:"X,attr"`
	Y string `xml:"Y,attr"`
}

type Points struct {
	Point []Point `xml:"Point"`
}

type Project struct {
	Id            string          `xml:"Id,attr"`
	IdFromSource  string          `xml:"IdFromSource,attr"`
	Name          string          `xml:"Name,attr"`
	Description   string          `xml:"Description,attr"`
	Tag           string          `xml:"Tag,attr"`
	ExternalCode  string          `xml:"ExternalCode,attr"`
	GraphicScheme []GraphicScheme `xml:"GraphicScheme"`
}

type Projects struct {
	Project []Project `xml:"Project"`
}

type ActiveGraphicScheme struct {
	Description   string `xml:"Description,attr"`
	ExternalCode  string `xml:"ExternalCode,attr"`
	GraphicScheme string `xml:"GraphicScheme,attr"`
	Id            string `xml:"Id,attr"`
	IdFromSource  string `xml:"IdFromSource,attr"`
	Name          string `xml:"Name,attr"`
	Tag           string `xml:"Tag,attr"`
}

type ActiveGraphicSchemes struct {
	ActiveGraphicScheme []ActiveGraphicScheme `xml:"ActiveGraphicScheme"`
}

type InitialCondition struct {
	Description  string     `xml:"Description,attr"`
	ExternalCode string     `xml:"ExternalCode,attr"`
	Id           string     `xml:"Id,attr"`
	IdFromSource string     `xml:"IdFromSource,attr"`
	Name         string     `xml:"Name,attr"`
	Tag          string     `xml:"Tag,attr"`
	Target       string     `xml:"Target,attr"`
	Property     []Property `xml:"Property"`
}

type InitialConditions struct {
	InitialCondition []InitialCondition `xml:"InitialCondition"`
}

type Measure struct {
	Description  string     `xml:"Description,attr"`
	ExternalCode string     `xml:"ExternalCode,attr"`
	Id           string     `xml:"Id,attr"`
	IdFromSource string     `xml:"IdFromSource,attr"`
	Name         string     `xml:"Name,attr"`
	Tag          string     `xml:"Tag,attr"`
	Target       string     `xml:"Target,attr"`
	Property     []Property `xml:"Property"`
}

type MeasureSlice struct {
	Description  string    `xml:"Description,attr"`
	ExternalCode string    `xml:"ExternalCode,attr"`
	Id           string    `xml:"Id,attr"`
	IdFromSource string    `xml:"IdFromSource,attr"`
	Name         string    `xml:"Name,attr"`
	Tag          string    `xml:"Tag,attr"`
	Time         string    `xml:"Time,attr"`
	Measure      []Measure `xml:"Measure"`
}

type MeasureSlices struct {
	MeasureSlice []MeasureSlice `xml:"MeasureSlice"`
}

type ObjectConditions struct{}

type Property struct {
	BooleanValue  string       `xml:"BooleanValue,attr"`
	Description   string       `xml:"Description,attr"`
	EnumItemValue string       `xml:"EnumItemValue,attr"`
	ExternalCode  string       `xml:"ExternalCode,attr"`
	Id            string       `xml:"Id,attr"`
	IdFromSource  string       `xml:"IdFromSource,attr"`
	IntegerValue  int          `xml:"IntegerValue,attr"`
	LinkValue     string       `xml:"LinkValue,attr"`
	Name          string       `xml:"Name,attr"`
	PointsValue   string       `xml:"PointsValue,attr"`
	RealValue     CommaFloat64 `xml:"RealValue,attr"`
	StringValue   string       `xml:"StringValue,attr"`
	Tag           string       `xml:"Tag,attr"`
	Template      string       `xml:"Template,attr"`
}

type Scenario struct {
	AuthorFullName              string               `xml:"AuthorFullName,attr"`
	Category                    string               `xml:"Category,attr"`
	Code                        string               `xml:"Code,attr"`
	ControlCommandPenaltyScore  string               `xml:"ControlCommandPenaltyScore,attr"`
	CreationTime                string               `xml:"CreationTime,attr"`
	Description                 string               `xml:"Description,attr"`
	District                    string               `xml:"District,attr"`
	Duration                    string               `xml:"Duration,attr"`
	ExternalCode                string               `xml:"ExternalCode,attr"`
	ForExamination              string               `xml:"ForExamination,attr"`
	ForQualification            string               `xml:"ForQualification,attr"`
	Id                          string               `xml:"Id,attr"`
	IdFromSource                string               `xml:"IdFromSource,attr"`
	IncludeTesting              string               `xml:"IncludeTesting,attr"`
	IsAutomationTracking        string               `xml:"IsAutomationTracking,attr"`
	IsLimitLoadTracking         string               `xml:"IsLimitLoadTracking,attr"`
	Name                        string               `xml:"Name,attr"`
	Number                      string               `xml:"Number,attr"`
	RedactingTime               string               `xml:"RedactingTime,attr"`
	RedactorFullName            string               `xml:"RedactorFullName,attr"`
	State                       string               `xml:"State,attr"`
	Tag                         string               `xml:"Tag,attr"`
	Workspace                   string               `xml:"Workspace,attr"`
	ActiveGraphicSchemes        ActiveGraphicSchemes `xml:"ActiveGraphicSchemes"`
	Documents                   struct{}             `xml:"Documents"`
	InitialConditions           InitialConditions    `xml:"InitialConditions"`
	MeasureSlices               MeasureSlices        `xml:"MeasureSlices"`
	NotTrackingAutomations      struct{}             `xml:"NotTrackingAutomations"`
	ObjectConditions            ObjectConditions     `xml:"ObjectConditions"`
	Plan                        struct{}             `xml:"Plan"`
	SimulationCheckpoints       struct{}             `xml:"SimulationCheckpoints"`
}

type Scenarios struct {
	Scenario []Scenario `xml:"Scenario"`
}

type Solution struct {
	Company                     string                      `xml:"Company,attr"`
	Description                 string                      `xml:"Description,attr"`
	ExternalCode                string                      `xml:"ExternalCode,attr"`
	Id                          string                      `xml:"Id,attr"`
	Name                        string                      `xml:"Name,attr"`
	StepIterationsCount         string                      `xml:"StepIterationsCount,attr"`
	Tag                         string                      `xml:"Tag,attr"`
	DataLayout                  string                      `json:"dataLayout,omitempty" xml:"-"`
	Automations                 Automations                 `xml:"Automations"`
	ControlCommands             ControlCommands             `xml:"ControlCommands"`
	Projects                    Projects                    `xml:"Projects"`
	Scenarios                   Scenarios                   `xml:"Scenarios"`
	TargetAreas                 TargetAreas                 `xml:"TargetAreas"`
	TechnologyObjectsLinks      TechnologyObjectsLinks      `xml:"TechnologyObjectsLinks"`
	TechnologyPassports         TechnologyPassports         `xml:"TechnologyPassports"`
	TechnologyStandaloneObjects TechnologyStandaloneObjects `xml:"TechnologyStandaloneObjects"`
	Workspaces                  Workspaces                  `xml:"Workspaces"`
}

const LeanSchemesLayoutV1 = "lean-schemes-v1"

func SchemeStub(scheme GraphicScheme) GraphicScheme {
	return GraphicScheme{
		Description:   scheme.Description,
		ExternalCode:  scheme.ExternalCode,
		Id:            scheme.Id,
		IdFromSource:  scheme.IdFromSource,
		Name:          scheme.Name,
		Tag:           scheme.Tag,
		Template:      scheme.Template,
		GraphicObject: []GraphicObject{},
	}
}

func (s *Solution) StripSchemeGraphicObjects() {
	if s == nil {
		return
	}
	for pi := range s.Projects.Project {
		for si := range s.Projects.Project[pi].GraphicScheme {
			s.Projects.Project[pi].GraphicScheme[si] = SchemeStub(s.Projects.Project[pi].GraphicScheme[si])
		}
	}
	s.DataLayout = LeanSchemesLayoutV1
}

func (s *Solution) HasSchemeGraphicObjects() bool {
	if s == nil {
		return false
	}
	for _, project := range s.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			if len(scheme.GraphicObject) > 0 {
				return true
			}
		}
	}
	return false
}

type TargetArea struct {
	Description  string `xml:"Description,attr"`
	ExternalCode string `xml:"ExternalCode,attr"`
	Id           string `xml:"Id,attr"`
	IdFromSource string `xml:"IdFromSource,attr"`
	Index        string `xml:"Index,attr"`
	Name         string `xml:"Name,attr"`
	Tag          string `xml:"Tag,attr"`
}

type TargetAreas struct {
	TargetArea []TargetArea `xml:"TargetArea"`
}

type TechnologyListItem struct {
	Description        string               `xml:"Description,attr"`
	ExternalCode       string               `xml:"ExternalCode,attr"`
	Id                 string               `xml:"Id,attr"`
	IdFromSource       string               `xml:"IdFromSource,attr"`
	Name               string               `xml:"Name,attr"`
	Tag                string               `xml:"Tag,attr"`
	Template           string               `xml:"Template,attr"`
	TechnologyProperty []TechnologyProperty `xml:"TechnologyProperty"`
}

type TechnologyObject struct {
	Description        string               `xml:"Description,attr"`
	ExternalCode       string               `xml:"ExternalCode,attr"`
	Id                 string               `xml:"Id,attr"`
	IdFromSource       string               `xml:"IdFromSource,attr"`
	Name               string               `xml:"Name,attr"`
	Tag                string               `xml:"Tag,attr"`
	Template           string               `xml:"Template,attr"`
	TechnologyProperty []TechnologyProperty `xml:"TechnologyProperty"`
	ParentName         string               `xml:"-"`
}

type TechnologyObjectLink struct {
	Description      string `xml:"Description,attr"`
	ExternalCode     string `xml:"ExternalCode,attr"`
	Id               string `xml:"Id,attr"`
	IdFromSource     string `xml:"IdFromSource,attr"`
	Name             string `xml:"Name,attr"`
	Tag              string `xml:"Tag,attr"`
	TechnologyObject string `xml:"TechnologyObject,attr"`
}

type TechnologyObjectsLink struct {
	Description       string `xml:"Description,attr"`
	ExternalCode      string `xml:"ExternalCode,attr"`
	Id                string `xml:"Id,attr"`
	IdFromSource      string `xml:"IdFromSource,attr"`
	Name              string `xml:"Name,attr"`
	Tag               string `xml:"Tag,attr"`
	TechnologyObject1 string `xml:"TechnologyObject1,attr"`
	TechnologyObject2 string `xml:"TechnologyObject2,attr"`
	Template          string `xml:"Template,attr"`
}

type TechnologyObjectsLinks struct {
	TechnologyObjectsLink []TechnologyObjectsLink `xml:"TechnologyObjectsLink"`
}

type TechnologyPassport struct {
	Description        string               `xml:"Description,attr"`
	ExternalCode       string               `xml:"ExternalCode,attr"`
	Id                 string               `xml:"Id,attr"`
	IdFromSource       string               `xml:"IdFromSource,attr"`
	Name               string               `xml:"Name,attr"`
	Tag                string               `xml:"Tag,attr"`
	Template           string               `xml:"Template,attr"`
	TechnologyProperty []TechnologyProperty `xml:"TechnologyProperty"`
}

type TechnologyPassports struct {
	TechnologyPassport []TechnologyPassport `xml:"TechnologyPassport"`
}

type TechnologyProperty struct {
	BooleanValue         string                 `xml:"BooleanValue,attr"`
	Description          string                 `xml:"Description,attr"`
	EnumItemValue        string                 `xml:"EnumItemValue,attr"`
	ExternalCode         string                 `xml:"ExternalCode,attr"`
	Id                   string                 `xml:"Id,attr"`
	IdFromSource         string                 `xml:"IdFromSource,attr"`
	IntegerValue         int                    `xml:"IntegerValue,attr"`
	LinkValue            string                 `xml:"LinkValue,attr"`
	Name                 string                 `xml:"Name,attr"`
	PointsValue          string                 `xml:"PointsValue,attr"`
	RealValue            CommaFloat64           `xml:"RealValue,attr"`
	StringValue          string                 `xml:"StringValue,attr"`
	Tag                  string                 `xml:"Tag,attr"`
	Template             string                 `xml:"Template,attr"`
	TechnologyListItem   []TechnologyListItem   `xml:"TechnologyListItem"`
	TechnologyObjectLink []TechnologyObjectLink `xml:"TechnologyObjectLink"`
	TechnologyPassport   []TechnologyPassport   `xml:"TechnologyPassport"`
}

type CommaFloat64 float64

func (cf *CommaFloat64) UnmarshalXMLAttr(attr xml.Attr) error {
	clean := strings.ReplaceAll(attr.Value, "\u00a0", " ")
	clean = strings.TrimSpace(clean)

	if clean == "" {
		*cf = 0
		return nil
	}

	normalized := strings.Replace(clean, ",", ".", 1)

	f, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		fmt.Printf("CommaFloat64: некорректное значение '%s', будет 0\n", attr.Value)
		*cf = 0
		return nil
	}

	*cf = CommaFloat64(f)
	return nil
}

type TechnologyStandaloneObject struct {
	Description        string               `xml:"Description,attr"`
	ExternalCode       string               `xml:"ExternalCode,attr"`
	Id                 string               `xml:"Id,attr"`
	IdFromSource       string               `xml:"IdFromSource,attr"`
	Name               string               `xml:"Name,attr"`
	Tag                string               `xml:"Tag,attr"`
	Template           string               `xml:"Template,attr"`
	TechnologyProperty []TechnologyProperty `xml:"TechnologyProperty"`
}

type TechnologyStandaloneObjects struct {
	TechnologyStandaloneObject []TechnologyStandaloneObject `xml:"TechnologyStandaloneObject"`
}

type Workspace struct {
	Description      string           `xml:"Description,attr"`
	ExternalCode     string           `xml:"ExternalCode,attr"`
	Id               string           `xml:"Id,attr"`
	IdFromSource     string           `xml:"IdFromSource,attr"`
	Index            string           `xml:"Index,attr"`
	Name             string           `xml:"Name,attr"`
	Tag              string           `xml:"Tag,attr"`
	ObjectReferences ObjectReferences `xml:"ObjectReferences"`
}

type Workspaces struct {
	Workspace []Workspace `xml:"Workspace"`
}

// Coord — координата на графической схеме.
type Coord struct {
	X float64
	Y float64
}

// ProfilePoint — точка профиля трубопровода (координата X; высота; давление).
type ProfilePoint struct {
	X        float64
	Elevation float64
	Pressure  float64
}

const (
	TemplateBridge = "Bridge"
	TemplatePipe   = "Pipe"
)

// ParseCoord парсит строковые координаты из XML.
func ParseCoord(x, y string) (Coord, error) {
	fx, err := parseFloat(x)
	if err != nil {
		return Coord{}, fmt.Errorf("X=%q: %w", x, err)
	}
	fy, err := parseFloat(y)
	if err != nil {
		return Coord{}, fmt.Errorf("Y=%q: %w", y, err)
	}
	return Coord{X: fx, Y: fy}, nil
}

func parseFloat(s string) (float64, error) {
	clean := strings.ReplaceAll(s, "\u00a0", " ")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return 0, nil
	}
	normalized := strings.Replace(clean, ",", ".", 1)
	return strconv.ParseFloat(normalized, 64)
}

// Anchor возвращает абсолютную точку привязки GraphicObject на схеме.
// AnchorPoint задаёт origin объекта; Points — смещения относительно этой точки.
func (g *GraphicObject) Anchor() (Coord, bool) {
	if len(g.AnchorPoint) == 0 {
		return Coord{}, false
	}
	c, err := ParseCoord(g.AnchorPoint[0].X, g.AnchorPoint[0].Y)
	if err != nil {
		return Coord{}, false
	}
	return c, true
}

// RelativePoints возвращает точки полилинии как смещения от AnchorPoint.
func (g *GraphicObject) RelativePoints() []Coord {
	var result []Coord
	for _, block := range g.Points {
		for _, p := range block.Point {
			c, err := ParseCoord(p.X, p.Y)
			if err != nil {
				continue
			}
			result = append(result, c)
		}
	}
	return result
}

// AbsolutePoints возвращает абсолютные координаты полилинии.
// Каждая Point в XML — смещение от AnchorPoint, а не накопительное смещение.
// Для Bridge/Pipe: первая точка обычно (0,0) и совпадает с якорем.
func (g *GraphicObject) AbsolutePoints() []Coord {
	anchor, ok := g.Anchor()
	if !ok {
		return nil
	}
	relative := g.RelativePoints()
	if len(relative) == 0 {
		return []Coord{anchor}
	}
	result := make([]Coord, len(relative))
	for i, p := range relative {
		result[i] = Coord{X: anchor.X + p.X, Y: anchor.Y + p.Y}
	}
	return result
}

// StartEnd возвращает начало и конец полилинии в абсолютных координатах.
func (g *GraphicObject) StartEnd() (start, end Coord, ok bool) {
	pts := g.AbsolutePoints()
	if len(pts) == 0 {
		return Coord{}, Coord{}, false
	}
	return pts[0], pts[len(pts)-1], true
}

// TechnologyTemplate возвращает Template первого TechnologyObject.
func (g *GraphicObject) TechnologyTemplate() string {
	if len(g.TechnologyObject) == 0 {
		return ""
	}
	return g.TechnologyObject[0].Template
}

// IsPipeLike возвращает true для Bridge и Pipe — объектов с полилинией.
func (g *GraphicObject) IsPipeLike() bool {
	switch g.TechnologyTemplate() {
	case TemplateBridge, TemplatePipe, "Pipeline":
		return true
	default:
		return len(g.RelativePoints()) > 0
	}
}

// LabelAnchors возвращает абсолютные позиции подписей GraphicLabel.
func (g *GraphicObject) LabelAnchors() []Coord {
	var result []Coord
	for _, block := range g.GraphicLabels {
		for _, label := range block.GraphicLabel {
			c, err := ParseCoord(label.AnchorPoint.X, label.AnchorPoint.Y)
			if err != nil {
				continue
			}
			result = append(result, c)
		}
	}
	return result
}

// PipeEndpoint — конец трубы (Bridge/Pipe) с привязкой к GraphicObject.
type PipeEndpoint struct {
	ObjectID string
	Object   *GraphicObject
	Point    Coord
	IsStart  bool
}

// PipeConnection — соединение двух труб в общей точке.
type PipeConnection struct {
	Endpoint1 PipeEndpoint
	Endpoint2 PipeEndpoint
}

// FindPipeConnections находит стыки труб (Bridge/Pipe), у которых совпадают
// начало или конец полилинии в пределах tolerance.
func (gs *GraphicScheme) FindPipeConnections(tolerance float64) []PipeConnection {
	var endpoints []PipeEndpoint
	for i := range gs.GraphicObject {
		obj := &gs.GraphicObject[i]
		if !obj.IsPipeLike() {
			continue
		}
		start, end, ok := obj.StartEnd()
		if !ok {
			continue
		}
		endpoints = append(endpoints,
			PipeEndpoint{ObjectID: obj.Id, Object: obj, Point: start, IsStart: true},
			PipeEndpoint{ObjectID: obj.Id, Object: obj, Point: end, IsStart: false},
		)
	}

	var connections []PipeConnection
	for i := 0; i < len(endpoints); i++ {
		for j := i + 1; j < len(endpoints); j++ {
			e1, e2 := endpoints[i], endpoints[j]
			if e1.ObjectID == e2.ObjectID {
				continue
			}
			if e1.Point.DistanceTo(e2.Point) <= tolerance {
				connections = append(connections, PipeConnection{Endpoint1: e1, Endpoint2: e2})
			}
		}
	}
	return connections
}

// DistanceTo возвращает расстояние между двумя координатами.
func (c Coord) DistanceTo(other Coord) float64 {
	dx := c.X - other.X
	dy := c.Y - other.Y
	return math.Hypot(dx, dy)
}

// ParseProfilePoints разбирает PointsValue из TechnologyProperty (формат: "X;elev;press X;elev;press ...").
func ParseProfilePoints(pointsValue string) ([]ProfilePoint, error) {
	pointsValue = strings.TrimSpace(pointsValue)
	if pointsValue == "" {
		return nil, nil
	}
	parts := strings.Fields(pointsValue)
	result := make([]ProfilePoint, 0, len(parts))
	for _, part := range parts {
		fields := strings.Split(part, ";")
		if len(fields) < 3 {
			return nil, fmt.Errorf("некорректная точка профиля: %q", part)
		}
		x, err := parseFloat(fields[0])
		if err != nil {
			return nil, fmt.Errorf("X в %q: %w", part, err)
		}
		elev, err := parseFloat(fields[1])
		if err != nil {
			return nil, fmt.Errorf("elevation в %q: %w", part, err)
		}
		press, err := parseFloat(fields[2])
		if err != nil {
			return nil, fmt.Errorf("pressure в %q: %w", part, err)
		}
		result = append(result, ProfilePoint{X: x, Elevation: elev, Pressure: press})
	}
	return result, nil
}

// ProfilePoints возвращает точки профиля из TechnologyProperty с Template=ProfilePoints.
func (to *TechnologyObject) ProfilePoints() ([]ProfilePoint, error) {
	for _, prop := range to.TechnologyProperty {
		if prop.Template == "ProfilePoints" && prop.PointsValue != "" {
			return ParseProfilePoints(prop.PointsValue)
		}
	}
	return nil, nil
}

// FindTechnologyObjectByID ищет TechnologyObject по Id среди всех GraphicObject в Solution.
func (s *Solution) FindTechnologyObjectByID(id string) *TechnologyObject {
	for _, project := range s.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for i := range scheme.GraphicObject {
				obj := &scheme.GraphicObject[i]
				for j := range obj.TechnologyObject {
					to := &obj.TechnologyObject[j]
					if to.Id == id {
						return to
					}
				}
			}
		}
	}
	return nil
}

// GraphicObjectsByTemplate возвращает все GraphicObject с указанным Template на схеме.
func (gs *GraphicScheme) GraphicObjectsByTemplate(template string) []*GraphicObject {
	var result []*GraphicObject
	for i := range gs.GraphicObject {
		if gs.GraphicObject[i].TechnologyTemplate() == template {
			result = append(result, &gs.GraphicObject[i])
		}
	}
	return result
}
