package evalgen

import (
	"go/format"
	"strings"
)

func formatted(source string) []byte {
	out, err := format.Source([]byte(source))
	if err != nil {
		panic("evalgen: generated source does not parse: " + err.Error() + "\n" + source)
	}
	return out
}

func (g spec) updateSecond() string { return "Update" + g.second }

// entityPackage is the data layer: the entity with its two fields, the nested type both share when the field
// needs one, and the functions the direction needs.
func entityPackage(g spec) []byte {
	var b strings.Builder
	b.WriteString("package " + g.route[g.cell.Depth].name + "\n\n")
	if g.cell.Field == "nested" {
		b.WriteString("type " + g.field + " struct {\n\tCity string\n\tCode string\n}\n\n")
	}
	b.WriteString("type " + g.entity + " struct {\n\tID string\n\tName string\n\t" + g.field + " " + g.fieldType("") + "\n\t" + g.second + " " + g.fieldType("") + "\n}\n\n")
	record := g.entity + "{ID: id, Name: \"" + sampleName + "\", " + g.field + ": " + qualify(g.shape.sample, "") + ", " + g.second + ": " + qualify(g.secondShape.sample, "") + "}"
	if g.cell.Direction == "read" {
		b.WriteString("func Load(id string) " + g.entity + " {\n\treturn " + record + "\n}\n")
	} else {
		b.WriteString("func Update(id string, value " + g.fieldType("") + ") " + g.entity + " {\n\trecord := " + record + "\n\trecord." + g.field + " = value\n\treturn record\n}\n")
		b.WriteString("\nfunc " + g.updateSecond() + "(id string, value " + g.fieldType("") + ") " + g.entity + " {\n\trecord := " + record + "\n\trecord." + g.second + " = value\n\treturn record\n}\n")
	}
	return formatted(b.String())
}

// passthroughPackage forwards the one function the direction needs to the next package on the route.
func passthroughPackage(g spec, index int) []byte {
	lower := g.route[index+1]
	var b strings.Builder
	b.WriteString("package " + g.route[index].name + "\n\nimport lower \"" + g.module + "/" + lower.dir + "\"\n\n")
	b.WriteString("type " + g.entity + " = lower." + g.entity + "\n")
	if g.cell.Field == "nested" {
		b.WriteString("type " + g.field + " = lower." + g.field + "\n")
	}
	if g.cell.Direction == "read" {
		b.WriteString("\nfunc Load(id string) " + g.entity + " {\n\treturn lower.Load(id)\n}\n")
	} else {
		b.WriteString("\nfunc Update(id string, value " + g.fieldType("") + ") " + g.entity + " {\n\treturn lower.Update(id, value)\n}\n")
		b.WriteString("\nfunc " + g.updateSecond() + "(id string, value " + g.fieldType("") + ") " + g.entity + " {\n\treturn lower." + g.updateSecond() + "(id, value)\n}\n")
	}
	return formatted(b.String())
}

func extraPackage(name, layer string) []byte {
	return formatted("package " + name + "\n\nfunc Describe() string {\n\treturn \"" + layer + "/" + name + "\"\n}\n")
}

// packageTest is the visible black-box test of one package; it calls only exported functions.
func packageTest(g spec, layer, name string, onRoute bool, index int) []byte {
	var b strings.Builder
	describeOnly := !onRoute || index == 0
	imports := "\t\"reflect\"\n\t\"testing\"\n"
	if describeOnly {
		imports = "\t\"testing\"\n"
	}
	b.WriteString("package " + name + "_test\n\nimport (\n" + imports + "\n\t\"" + g.module + "/internal/" + layer + "/" + name + "\"\n)\n\n")
	if describeOnly {
		b.WriteString("func TestDescribeNamesThePackage(t *testing.T) {\n\tif got := " + name + ".Describe(); got != \"" + layer + "/" + name + "\" {\n\t\tt.Fatalf(\"Describe() = %q\", got)\n\t}\n}\n")
		return formatted(b.String())
	}
	if g.cell.Direction == "read" {
		b.WriteString("func TestLoadReturnsTheSampleRecord(t *testing.T) {\n\trecord := " + name + ".Load(\"" + recordID + "\")\n\tif record.ID != \"" + recordID + "\" || !reflect.DeepEqual(record." + g.field + ", " + qualify(g.shape.sample, name+".") + ") || !reflect.DeepEqual(record." + g.second + ", " + qualify(g.secondShape.sample, name+".") + ") {\n\t\tt.Fatalf(\"Load = %+v\", record)\n\t}\n}\n")
	} else {
		b.WriteString("func TestUpdateReplacesOnlyTheField(t *testing.T) {\n\trecord := " + name + ".Update(\"" + recordID + "\", " + qualify(g.shape.value, name+".") + ")\n\tif record.ID != \"" + recordID + "\" || record.Name != \"" + sampleName + "\" || !reflect.DeepEqual(record." + g.field + ", " + qualify(g.shape.value, name+".") + ") {\n\t\tt.Fatalf(\"Update = %+v\", record)\n\t}\n}\n")
		b.WriteString("\nfunc TestUpdateSecondReplacesOnlyTheSecondField(t *testing.T) {\n\trecord := " + name + "." + g.updateSecond() + "(\"" + recordID + "\", " + qualify(g.secondShape.value, name+".") + ")\n\tif record.ID != \"" + recordID + "\" || record.Name != \"" + sampleName + "\" || !reflect.DeepEqual(record." + g.second + ", " + qualify(g.secondShape.value, name+".") + ") || !reflect.DeepEqual(record." + g.field + ", " + qualify(g.shape.sample, name+".") + ") {\n\t\tt.Fatalf(\"UpdateSecond = %+v\", record)\n\t}\n}\n")
	}
	return formatted(b.String())
}
