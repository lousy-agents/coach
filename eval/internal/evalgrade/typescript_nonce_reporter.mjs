import fs from "node:fs";
import path from "node:path";

const nonce = fs.readFileSync(3, "utf8").trim();
fs.closeSync(3);

const statuses = { "test:pass": "pass", "test:fail": "fail" };

function status(event) {
  if (event.data.skip) return "skipped";
  if (event.data.todo) return "todo";
  return statuses[event.type];
}

export default async function* (source) {
  const chain = [];
  for await (const event of source) {
    if (event.type === "test:start") {
      chain.length = event.data.nesting;
      chain.push(event.data.name);
    }
    if (!(event.type in statuses) || event.data.details?.type === "suite") continue;
    const names = chain.slice(0, event.data.nesting).concat(event.data.name);
    const file = path.relative(process.cwd(), event.data.file ?? "").split(path.sep).join("/");
    fs.writeSync(4, JSON.stringify({ nonce, id: `${file}::${names.join(" > ")}`, status: status(event) }) + "\n");
  }
}
