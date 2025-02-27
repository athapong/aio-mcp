package visualizer

import (
	"bytes"
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"

	"github.com/athapong/aio-mcp/pkg/graph"
)

// The HTML template for D3.js visualization
const d3Template = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Knowledge Graph Visualization</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <style>
        body { 
            margin: 0;
            font-family: Arial, sans-serif;
        }
        #graph {
            width: 100%;
            height: 100vh;
            background-color: #f5f5f5;
        }
        .node {
            stroke: #fff;
            stroke-width: 1.5px;
        }
        .link {
            stroke: #999;
            stroke-opacity: 0.6;
        }
        .node-label {
            font-size: 10px;
            pointer-events: none;
        }
        .controls {
            position: absolute;
            top: 10px;
            left: 10px;
            background-color: rgba(255,255,255,0.8);
            padding: 10px;
            border-radius: 5px;
            box-shadow: 0 0 10px rgba(0,0,0,0.1);
        }
    </style>
</head>
<body>
    <div id="graph"></div>
    <div class="controls">
        <h3>Knowledge Graph</h3>
        <p>Nodes: {{.NodeCount}}, Edges: {{.EdgeCount}}</p>
        <div>
            <label for="node-type-filter">Filter by node type:</label>
            <select id="node-type-filter">
                <option value="all">All Types</option>
            </select>
        </div>
    </div>

    <script>
        // Graph data
        const graphData = {{.GraphData}};
        
        // Initialize the force simulation
        const simulation = d3.forceSimulation(graphData.nodes)
            .force("link", d3.forceLink(graphData.edges).id(d => d.id).distance(100))
            .force("charge", d3.forceManyBody().strength(-300))
            .force("center", d3.forceCenter(window.innerWidth / 2, window.innerHeight / 2));

        // Create SVG element
        const svg = d3.select("#graph")
            .append("svg")
            .attr("width", "100%")
            .attr("height", "100%")
            .call(d3.zoom().on("zoom", (event) => {
                g.attr("transform", event.transform);
            }));

        const g = svg.append("g");

        // Define node colors based on types
        const nodeTypes = [...new Set(graphData.nodes.map(node => node.type))];
        const colorScale = d3.scaleOrdinal(d3.schemeCategory10).domain(nodeTypes);

        // Add node types to filter dropdown
        nodeTypes.forEach(type => {
            d3.select("#node-type-filter")
                .append("option")
                .attr("value", type)
                .text(type);
        });

        // Create links
        const link = g.append("g")
            .selectAll("line")
            .data(graphData.edges)
            .enter()
            .append("line")
            .attr("class", "link")
            .attr("stroke-width", d => Math.sqrt(d.weight) * 2);

        // Create nodes
        const node = g.append("g")
            .selectAll("circle")
            .data(graphData.nodes)
            .enter()
            .append("circle")
            .attr("class", "node")
            .attr("r", 8)
            .attr("fill", d => colorScale(d.type))
            .call(d3.drag()
                .on("start", dragstarted)
                .on("drag", dragged)
                .on("end", dragended));

        // Add labels to nodes
        const label = g.append("g")
            .selectAll("text")
            .data(graphData.nodes)
            .enter()
            .append("text")
            .attr("class", "node-label")
            .attr("dx", 12)
            .attr("dy", ".35em")
            .text(d => d.label);

        // Node tooltip
        node.append("title")
            .text(d => d.label + " (" + d.type + ")");

        // Link tooltip
        link.append("title")
            .text(d => d.type);

        // Update positions on simulation tick
        simulation.on("tick", () => {
            link
                .attr("x1", d => d.source.x)
                .attr("y1", d => d.source.y)
                .attr("x2", d => d.target.x)
                .attr("y2", d => d.target.y);

            node
                .attr("cx", d => d.x)
                .attr("cy", d => d.y);

            label
                .attr("x", d => d.x)
                .attr("y", d => d.y);
        });

        // Node type filter
        d3.select("#node-type-filter").on("change", function() {
            const selectedType = this.value;
            
            if (selectedType === "all") {
                node.style("visibility", "visible");
                link.style("visibility", "visible");
                label.style("visibility", "visible");
                return;
            }
            
            // Hide nodes that don't match the selected type
            node.style("visibility", d => d.type === selectedType ? "visible" : "hidden");
            
            // Hide labels for hidden nodes
            label.style("visibility", d => d.type === selectedType ? "visible" : "hidden");
            
            // Hide links that don't connect to visible nodes
            link.style("visibility", d => {
                const sourceVisible = d.source.type === selectedType;
                const targetVisible = d.target.type === selectedType;
                return sourceVisible || targetVisible ? "visible" : "hidden";
            });
        });

        // Drag functions
        function dragstarted(event, d) {
            if (!event.active) simulation.alphaTarget(0.3).restart();
            d.fx = d.x;
            d.fy = d.y;
        }

        function dragged(event, d) {
            d.fx = event.x;
            d.fy = event.y;
        }

        function dragended(event, d) {
            if (!event.active) simulation.alphaTarget(0);
            d.fx = null;
            d.fy = null;
        }
    </script>
</body>
</html>
`

// D3Visualizer creates D3.js-based visualizations of knowledge graphs
type D3Visualizer struct {
	outputPath string
}

// NewD3Visualizer creates a new D3.js visualizer
func NewD3Visualizer(outputPath string) *D3Visualizer {
	return &D3Visualizer{
		outputPath: outputPath,
	}
}

// Visualize generates an HTML visualization of the knowledge graph
func (v *D3Visualizer) Visualize(graph *graph.KnowledgeGraphData) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(v.outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Convert graph data to JSON for the template
	graphData, err := json.Marshal(graph)
	if err != nil {
		return err
	}

	// Parse template
	tmpl, err := template.New("d3").Parse(d3Template)
	if err != nil {
		return err
	}

	// Prepare template data
	data := struct {
		GraphData string
		NodeCount int
		EdgeCount int
	}{
		GraphData: string(graphData),
		NodeCount: len(graph.Nodes),
		EdgeCount: len(graph.Edges),
	}

	// Render template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(v.outputPath, buf.Bytes(), 0644)
}
