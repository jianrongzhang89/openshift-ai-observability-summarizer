/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dependencies

import (
	"fmt"
	"sort"
)

// ComponentType represents a deployable component
type ComponentType string

const (
	// Tier 0: Observability Stack
	ComponentMinIO         ComponentType = "minio"
	ComponentTempo         ComponentType = "tempo"
	ComponentLoki          ComponentType = "loki"
	ComponentOTelCollector ComponentType = "otel-collector"
	ComponentKorrel8r      ComponentType = "korrel8r"

	// Tier 1: RAG Backend
	ComponentPostgreSQL    ComponentType = "postgresql"
	ComponentLlamaStack    ComponentType = "llama-stack"
	ComponentIngestionPipe ComponentType = "ingestion-pipeline"

	// Tier 2: MCP Server
	ComponentMCPServer ComponentType = "mcp-server"

	// Tier 3: Console Plugin and UI
	ComponentConsolePlugin ComponentType = "console-plugin"
	ComponentUI            ComponentType = "ui"
)

// ComponentState represents the deployment state of a component
type ComponentState string

const (
	StateNotDeployed ComponentState = "NotDeployed"
	StateDeploying   ComponentState = "Deploying"
	StateReady       ComponentState = "Ready"
	StateFailed      ComponentState = "Failed"
)

// Component represents a node in the dependency graph
type Component struct {
	Type         ComponentType
	Dependencies []ComponentType
	State        ComponentState
	Tier         int
}

// DependencyGraph manages component dependencies and deployment order
type DependencyGraph struct {
	components map[ComponentType]*Component
}

// NewDependencyGraph creates a new dependency graph with predefined component relationships
func NewDependencyGraph() *DependencyGraph {
	g := &DependencyGraph{
		components: make(map[ComponentType]*Component),
	}

	// Tier 0: Observability Stack (no dependencies)
	g.addComponent(ComponentMinIO, []ComponentType{}, 0)
	g.addComponent(ComponentTempo, []ComponentType{ComponentMinIO}, 0)
	g.addComponent(ComponentLoki, []ComponentType{ComponentMinIO}, 0)
	g.addComponent(ComponentOTelCollector, []ComponentType{ComponentTempo, ComponentLoki}, 0)
	g.addComponent(ComponentKorrel8r, []ComponentType{ComponentOTelCollector}, 0)

	// Tier 1: RAG Backend (depends on observability stack)
	g.addComponent(ComponentPostgreSQL, []ComponentType{ComponentOTelCollector}, 1)
	g.addComponent(ComponentLlamaStack, []ComponentType{ComponentPostgreSQL, ComponentOTelCollector}, 1)
	g.addComponent(ComponentIngestionPipe, []ComponentType{ComponentPostgreSQL, ComponentLlamaStack}, 1)

	// Tier 2: MCP Server (depends on Korrel8r and optionally RAG backend)
	g.addComponent(ComponentMCPServer, []ComponentType{ComponentKorrel8r, ComponentIngestionPipe, ComponentLlamaStack}, 2)

	// Tier 3: Console Plugin and UI (depends on MCP Server)
	g.addComponent(ComponentConsolePlugin, []ComponentType{ComponentMCPServer}, 3)
	g.addComponent(ComponentUI, []ComponentType{ComponentMCPServer}, 3)

	return g
}

// addComponent adds a component to the graph
func (g *DependencyGraph) addComponent(componentType ComponentType, deps []ComponentType, tier int) {
	g.components[componentType] = &Component{
		Type:         componentType,
		Dependencies: deps,
		State:        StateNotDeployed,
		Tier:         tier,
	}
}

// SetComponentState updates the state of a component
func (g *DependencyGraph) SetComponentState(componentType ComponentType, state ComponentState) error {
	comp, exists := g.components[componentType]
	if !exists {
		return fmt.Errorf("component %s not found in graph", componentType)
	}
	comp.State = state
	return nil
}

// RemoveComponent removes a component from the graph and cleans up dependencies
func (g *DependencyGraph) RemoveComponent(componentType ComponentType) {
	// Remove the component itself
	delete(g.components, componentType)

	// Remove it from dependencies of other components
	for _, comp := range g.components {
		newDeps := []ComponentType{}
		for _, dep := range comp.Dependencies {
			if dep != componentType {
				newDeps = append(newDeps, dep)
			}
		}
		comp.Dependencies = newDeps
	}
}

// GetComponentState returns the current state of a component
func (g *DependencyGraph) GetComponentState(componentType ComponentType) (ComponentState, error) {
	comp, exists := g.components[componentType]
	if !exists {
		return "", fmt.Errorf("component %s not found in graph", componentType)
	}
	return comp.State, nil
}

// AreDependenciesReady checks if all dependencies of a component are ready
func (g *DependencyGraph) AreDependenciesReady(componentType ComponentType) (bool, []ComponentType, error) {
	comp, exists := g.components[componentType]
	if !exists {
		return false, nil, fmt.Errorf("component %s not found in graph", componentType)
	}

	var notReady []ComponentType
	for _, dep := range comp.Dependencies {
		depComp, exists := g.components[dep]
		if !exists {
			return false, nil, fmt.Errorf("dependency %s not found in graph", dep)
		}
		if depComp.State != StateReady {
			notReady = append(notReady, dep)
		}
	}

	return len(notReady) == 0, notReady, nil
}

// GetNextDeployableComponents returns components whose dependencies are satisfied
// and are ready to be deployed
func (g *DependencyGraph) GetNextDeployableComponents() []ComponentType {
	var deployable []ComponentType

	for componentType, comp := range g.components {
		// Skip if already deployed/deploying
		if comp.State != StateNotDeployed {
			continue
		}

		// Check if dependencies are ready
		ready, _, err := g.AreDependenciesReady(componentType)
		if err != nil {
			continue
		}

		if ready {
			deployable = append(deployable, componentType)
		}
	}

	// Sort by tier to ensure consistent ordering
	sort.Slice(deployable, func(i, j int) bool {
		return g.components[deployable[i]].Tier < g.components[deployable[j]].Tier
	})

	return deployable
}

// GetDeploymentOrder returns the complete topological order for deployment
func (g *DependencyGraph) GetDeploymentOrder() ([]ComponentType, error) {
	// Create a copy of the graph for topological sort
	inDegree := make(map[ComponentType]int)
	adjList := make(map[ComponentType][]ComponentType)

	// Initialize in-degree and adjacency list
	for componentType, comp := range g.components {
		inDegree[componentType] = len(comp.Dependencies)
		for _, dep := range comp.Dependencies {
			adjList[dep] = append(adjList[dep], componentType)
		}
	}

	// Kahn's algorithm for topological sort
	var queue []ComponentType
	var order []ComponentType

	// Find all nodes with in-degree 0
	for componentType, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, componentType)
		}
	}

	// Sort initial queue by tier for consistent ordering
	sort.Slice(queue, func(i, j int) bool {
		return g.components[queue[i]].Tier < g.components[queue[j]].Tier
	})

	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		// Reduce in-degree for neighbors
		for _, neighbor := range adjList[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}

		// Re-sort queue by tier
		sort.Slice(queue, func(i, j int) bool {
			return g.components[queue[i]].Tier < g.components[queue[j]].Tier
		})
	}

	// Check for cycles
	if len(order) != len(g.components) {
		return nil, fmt.Errorf("dependency graph contains cycles")
	}

	return order, nil
}

// IsDeploymentComplete checks if all components are in Ready state
func (g *DependencyGraph) IsDeploymentComplete() bool {
	for _, comp := range g.components {
		if comp.State != StateReady {
			return false
		}
	}
	return true
}

// GetComponentsByTier returns all components in a specific tier
func (g *DependencyGraph) GetComponentsByTier(tier int) []ComponentType {
	var components []ComponentType
	for componentType, comp := range g.components {
		if comp.Tier == tier {
			components = append(components, componentType)
		}
	}

	sort.Slice(components, func(i, j int) bool {
		return string(components[i]) < string(components[j])
	})

	return components
}

// GetComponentTier returns the tier of a component
func (g *DependencyGraph) GetComponentTier(componentType ComponentType) (int, error) {
	comp, exists := g.components[componentType]
	if !exists {
		return -1, fmt.Errorf("component %s not found in graph", componentType)
	}
	return comp.Tier, nil
}

// GetAllComponents returns all component types in the graph
func (g *DependencyGraph) GetAllComponents() []ComponentType {
	var components []ComponentType
	for componentType := range g.components {
		components = append(components, componentType)
	}

	sort.Slice(components, func(i, j int) bool {
		tierI := g.components[components[i]].Tier
		tierJ := g.components[components[j]].Tier
		if tierI != tierJ {
			return tierI < tierJ
		}
		return string(components[i]) < string(components[j])
	})

	return components
}

// GetFailedComponents returns all components in Failed state
func (g *DependencyGraph) GetFailedComponents() []ComponentType {
	var failed []ComponentType
	for componentType, comp := range g.components {
		if comp.State == StateFailed {
			failed = append(failed, componentType)
		}
	}
	return failed
}

// Reset resets all component states to NotDeployed
func (g *DependencyGraph) Reset() {
	for _, comp := range g.components {
		comp.State = StateNotDeployed
	}
}
