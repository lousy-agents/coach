import { KIND_POSSIBLE_CALL_REACHABILITY, } from "./protocol.js";
import { CONFIDENCE_RESOLVED_DIRECT, REACHABILITY_ALGORITHM, REACHABILITY_BACKEND, } from "./reachability-registry.js";
export class ReachabilityAccumulator {
    callGraph = [];
    facts = [];
    diagnostics = [];
    seenSources;
    factKeys;
    seenGapSites;
    constructor(seenSources, factKeys, seenGapSites) {
        this.seenSources = seenSources;
        this.factKeys = factKeys;
        this.seenGapSites = seenGapSites;
    }
    noteSource(sourceId) {
        if (this.seenSources.has(sourceId))
            return false;
        this.seenSources.add(sourceId);
        return true;
    }
    recordFact(sourceId, sinkId) {
        const factKey = `${sourceId}->${sinkId}`;
        if (this.factKeys.has(factKey))
            return;
        this.factKeys.add(factKey);
        this.callGraph.push({ from: sourceId, to: sinkId });
        this.facts.push({
            id: `reach:${factKey}@${REACHABILITY_ALGORITHM}`,
            kind: KIND_POSSIBLE_CALL_REACHABILITY,
            confidence: CONFIDENCE_RESOLVED_DIRECT,
            source: sourceId,
            sink: sinkId,
            path: [{ node_id: sourceId }, { node_id: sinkId }],
            algorithm_version: REACHABILITY_ALGORITHM,
            backend: REACHABILITY_BACKEND,
        });
    }
    recordGap(code, message, node, sf, snapshot) {
        const repoPath = snapshot.toRepoPath(sf.path);
        const { line } = sf.getLineAndCharacterOfPosition(node.getStart(sf));
        const site = `${repoPath ?? sf.fileName}:${line + 1}`;
        const key = `${code}|${site}`;
        if (this.seenGapSites.has(key))
            return;
        this.seenGapSites.add(key);
        this.diagnostics.push({ code, message, path: repoPath });
    }
}
//# sourceMappingURL=reachability-accumulator.js.map