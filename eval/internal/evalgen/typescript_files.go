package evalgen

// The project files of a TypeScript program stay inside what the layer grader reads: one root tsconfig.json with no
// baseUrl, paths, extends or references, and a package.json with no imports or exports.
const tsConfigJSON = `{
  "compilerOptions": {
    "target": "es2022",
    "module": "nodenext",
    "moduleResolution": "nodenext",
    "strict": true,
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "erasableSyntaxOnly": true,
    "verbatimModuleSyntax": true,
    "skipLibCheck": true,
    "types": []
  },
  "include": ["src"],
  "exclude": ["src/**/*.test.ts"]
}
`

func packageJSON(name string) []byte {
	return []byte("{\n  \"name\": \"" + name + "\",\n  \"private\": true,\n  \"type\": \"module\"\n}\n")
}
