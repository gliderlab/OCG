# Knowledge Graph

Structured entity-relation memory storage.

---

## Overview

The Knowledge Graph (KG) allows storing and retrieving information as structured entities and relationships. This complements vector memory by providing exact facts and connections between concepts.

---

## Architecture

- **Entities**: Nodes in the graph (e.g., "OpenClaw-Go", "Jacker", "SQLite").
- **Relations**: Edges connecting entities (e.g., "Jacker" -> "works_on" -> "OpenClaw-Go").
- **Storage**: SQLite-backed tables `memory_entities` and `memory_relations`.

---

## Tooling

The Knowledge Graph is managed via the `memory_graph` tool.

### Actions

- `add_entity`: Create or update a node with a name, type, and description.
- `add_relation`: Create a weighted edge between two existing nodes.
- `get_entity`: Retrieve details about a specific node.
- `search_relations`: Find all connections for a specific node.

---

## Usage

### Add Entity

```bash
memory_graph(action="add_entity", name="OCG", entity_type="Project", description="Go rewrite of OpenClaw")
```

### Add Relation

```bash
memory_graph(action="add_relation", source="Jacker", target="OCG", relation="leads", weight=1.0)
```

### Search Relations

```bash
memory_graph(action="search_relations", name="Jacker")
```

---

## Hybrid Retrieval

The Knowledge Graph can be used alongside Vector Memory to provide a more comprehensive context for the AI agent.

---

## See Also

- [Vector Memory](vector.md)
- [Memory Tools](../../05-tools/memory.md)
