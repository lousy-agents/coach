package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsQualifiedMethodName names a method as the context of the functions
// inside it: "Class.name", with an accessor's kind in front of its name
// ("Class.get name"), so same-named methods of different classes, and a
// getter and setter of one name, are different holders. A method outside any
// named class, such as an object literal method, keeps its bare name.
func tsQualifiedMethodName(method, nameNode engine.Node, source []byte) string {
	name := nameNode.Utf8Text(source)
	if kind := tsAccessorKind(method, nameNode); kind != "" {
		name = kind + " " + name
	}
	if class := tsEnclosingClassName(method, source); class != "" {
		return class + "." + name
	}
	return name
}

// tsAccessorKind returns "get" or "set" when the keyword precedes the
// method's name, and "" otherwise. A method that is itself called get or set
// has no keyword before its name.
func tsAccessorKind(method, nameNode engine.Node) string {
	count := method.ChildCount()
	for i := 0; i < count; i++ {
		child := method.Child(i)
		if child.StartByte() >= nameNode.StartByte() {
			break
		}
		if kind := child.Kind(); kind == "get" || kind == "set" {
			return kind
		}
	}
	return ""
}

// tsEnclosingClassName returns the name of the class whose body declares
// method: the class's own name, or for an anonymous class expression the
// name it is held under. It returns "" for a method outside a class body and
// for an anonymous class held under no name.
func tsEnclosingClassName(method engine.Node, source []byte) string {
	body := method.Parent()
	if body == nil || body.Kind() != "class_body" {
		return ""
	}
	class := body.Parent()
	if class == nil {
		return ""
	}
	if nameNode := class.ChildByFieldName("name"); nameNode != nil {
		return nameNode.Utf8Text(source)
	}
	return tsAssignedName(class, source)
}
