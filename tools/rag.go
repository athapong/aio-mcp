package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/athapong/aio-mcp/services"
	"github.com/athapong/aio-mcp/util"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pkoukk/tiktoken-go"
	"github.com/qdrant/go-client/qdrant"
	"github.com/sashabaranov/go-openai"
)

// Update model dimensions mapping to include commonly used compatible models
var embeddingModelDimensions = map[openai.EmbeddingModel]uint64{
	openai.AdaEmbeddingV2:  1536,
	openai.SmallEmbedding3: 512,
	openai.LargeEmbedding3: 2048,
	"baai/bge-base-en":     768,  // BGE base model
	"baai/bge-large-en":    1024, // BGE large model
	"codesmart.embedding":  1536, // CodeSmart embedding model
}

// Update validation function to work with EmbeddingModel
func validateEmbeddingModel(modelStr string) (openai.EmbeddingModel, uint64, error) {
	model := openai.EmbeddingModel(modelStr)
	if dimensions, ok := embeddingModelDimensions[model]; ok {
		return model, dimensions, nil
	}
	return "", 0, fmt.Errorf("unsupported embedding model: %s. Supported models: %s",
		modelStr,
		"text-embedding-ada-002, text-embedding-3-small, text-embedding-3-large, baai/bge-base-en, baai/bge-large-en, codesmart.embedding")
}

var qdrantClient = sync.OnceValue(func() *qdrant.Client {

	host := os.Getenv("QDRANT_HOST")
	port := os.Getenv("QDRANT_PORT")
	apiKey := os.Getenv("QDRANT_API_KEY")
	if host == "" || port == "" || apiKey == "" {
		panic("QDRANT_HOST, QDRANT_PORT, or QDRANT_API_KEY is not set, please set it in MCP Config")
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		panic(fmt.Sprintf("failed to parse QDRANT_PORT: %v", err))
	}

	if apiKey == "" {
		panic("QDRANT_API_KEY is not set")
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   host,
		Port:   portInt,
		APIKey: apiKey,
		UseTLS: true,
	})

	if err != nil {
		panic(fmt.Sprintf("failed to connect to Qdrant: %v", err))
	}

	return client
})

func RegisterRagTools(s *server.MCPServer) {
	indexContentTool := mcp.NewTool("RAG_memory_index_content",
		mcp.WithDescription("Index a content into memory, can be inserted or updated"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Memory collection name")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("content file path")),
		mcp.WithString("payload", mcp.Required(), mcp.Description("Plain text payload")),
		mcp.WithString("model", mcp.Description("Embedding model to use (default: text-embedding-3-large)")),
	)

	indexFileTool := mcp.NewTool("RAG_memory_index_file",
		mcp.WithDescription("Index a local file into memory"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Memory collection name")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Path to the local file to be indexed")),
	)

	createCollectionTool := mcp.NewTool("RAG_memory_create_collection",
		mcp.WithDescription("Create a new vector collection in memory"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Memory collection name")),
		mcp.WithString("model", mcp.Description("Embedding model to use (default: text-embedding-3-large)")),
	)

	deleteCollectionTool := mcp.NewTool("RAG_memory_delete_collection",
		mcp.WithDescription("Delete a vector collection in memory"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Memory collection name")),
	)

	listCollectionTool := mcp.NewTool("RAG_memory_list_collections",
		mcp.WithDescription("List all vector collections in memory"),
	)

	searchTool := mcp.NewTool("RAG_memory_search",
		mcp.WithDescription("Search for memory in a collection based on a query"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Memory collection name")),
		mcp.WithString("query", mcp.Required(), mcp.Description("search query, should be a keyword")),
		mcp.WithString("model", mcp.Description("Embedding model to use (default: text-embedding-3-large)")),
	)

	deleteIndexByFilePathTool := mcp.NewTool("RAG_memory_delete_index_by_filepath",
		mcp.WithDescription("Delete a vector index by filePath"),
		mcp.WithString("collection", mcp.Required(), mcp.Description("Memory collection name")),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Path to the local file to be deleted")),
	)

	s.AddTool(createCollectionTool, util.ErrorGuard(createCollectionHandler))
	s.AddTool(deleteCollectionTool, util.ErrorGuard(deleteCollectionHandler))
	s.AddTool(listCollectionTool, util.ErrorGuard(listCollectionHandler))
	s.AddTool(indexContentTool, util.ErrorGuard(indexContentHandler))
	s.AddTool(searchTool, util.ErrorGuard(vectorSearchHandler))
	s.AddTool(indexFileTool, util.ErrorGuard(indexFileHandler))
	s.AddTool(deleteIndexByFilePathTool, util.ErrorGuard(deleteIndexByFilePathHandler))
}

func deleteIndexByFilePathHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	collection := arguments["collection"].(string)
	filePath := arguments["filePath"].(string)
	ctx := context.Background()

	// Delete points by IDs using PointSelector
	pointsSelector := &qdrant.PointsSelector{
		PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
			Filter: &qdrant.Filter{
				Must: []*qdrant.Condition{
					{
						ConditionOneOf: &qdrant.Condition_Field{
							Field: &qdrant.FieldCondition{
								Key: "filePath",
								Match: &qdrant.Match{
									MatchValue: &qdrant.Match_Text{
										Text: filePath,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	deleteResp, err := qdrantClient().Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points:         pointsSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete points for filePath %s: %v", filePath, err)
	}

	result := fmt.Sprintf("Successfully deleted points for filePath: %s\nOperation ID: %d\nStatus: %s", filePath, deleteResp.OperationId, deleteResp.Status)
	return mcp.NewToolResultText(result), nil
}

func indexFileHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	collection := arguments["collection"].(string)
	filePath := arguments["filePath"].(string)

	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Prepare arguments for vectorUpsertHandler
	upsertArgs := map[string]interface{}{
		"collection": collection,
		"filePath":   filePath,
		"payload":    string(content), // Convert content to string
	}

	// Call vectorUpsertHandler
	return indexContentHandler(upsertArgs)
}

func listCollectionHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	ctx := context.Background()
	collections, err := qdrantClient().ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}
	return mcp.NewToolResultText(fmt.Sprintf("Collections: %v", collections)), nil
}

// Update createCollectionHandler to always use codesmart.embedding
func createCollectionHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	collection := arguments["collection"].(string)
	modelStr := "codesmart.embedding" // Always use codesmart.embedding as default

	if modelArg, ok := arguments["model"].(string); ok && modelArg != "" {
		embModel, _, err := validateEmbeddingModel(modelArg)
		if err != nil {
			return nil, err
		}
		modelStr = string(embModel)
	}

	ctx := context.Background()

	// Check if collection already exists
	collectionInfo, err := qdrantClient().GetCollectionInfo(ctx, collection)
	if err == nil && collectionInfo != nil {
		return nil, fmt.Errorf("collection %s already exists", collection)
	}

	// Get dimensions for the model
	dimensions := embeddingModelDimensions[openai.EmbeddingModel(modelStr)]

	// Create collection with configuration for the selected model
	err = qdrantClient().CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collection,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     dimensions,
					Distance: qdrant.Distance_Cosine,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %v", err)
	}

	result := fmt.Sprintf("Successfully created collection: %s with model: %s", collection, modelStr)
	return mcp.NewToolResultText(result), nil
}

func deleteCollectionHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	collection := arguments["collection"].(string)
	ctx := context.Background()

	// Check if collection exists
	collectionInfo, err := qdrantClient().GetCollectionInfo(ctx, collection)
	if err != nil || collectionInfo == nil {
		return nil, fmt.Errorf("collection %s does not exist", collection)
	}

	// Delete collection
	err = qdrantClient().DeleteCollection(ctx, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to delete collection: %v", err)
	}

	result := fmt.Sprintf("Successfully deleted collection: %s", collection)
	return mcp.NewToolResultText(result), nil
}

// Update indexContentHandler to use codesmart.embedding by default
func indexContentHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	collection := arguments["collection"].(string)
	filePath := arguments["filePath"].(string)
	payload := arguments["payload"].(string)

	// Always default to codesmart.embedding
	modelStr := "codesmart.embedding"
	if modelArg, ok := arguments["model"].(string); ok && modelArg != "" {
		embModel, _, err := validateEmbeddingModel(modelArg)
		if err != nil {
			return nil, err
		}
		modelStr = string(embModel)
	}

	// Split content into chunks
	chunks, err := splitIntoChunks(payload, filePath) // Implement chunking logic
	if err != nil {
		return nil, fmt.Errorf("failed to split into chunks: %v", err)
	}

	var points []*qdrant.PointStruct
	for i, chunk := range chunks {
		// Generate embeddings for each chunk using selected model
		resp, err := services.DefaultOpenAIClient().CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
			Input: []string{chunk},
			Model: openai.EmbeddingModel(modelStr),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings: %v", err)
		}

		// Create point for each chunk
		point := &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(uuid.NewSHA1(uuid.NameSpaceURL, []byte(filePath+strconv.Itoa(i))).String()),
			Vectors: qdrant.NewVectors(resp.Data[0].Embedding...),
			Payload: qdrant.NewValueMap(map[string]any{
				"filePath":   filePath,
				"content":    chunk,
				"chunkIndex": i,
				"model":      modelStr, // Store the model used for embedding
			}),
		}
		points = append(points, point)
	}

	ctx := context.Background()
	waitUpsert := true

	// Upsert all chunks
	upsertResp, err := qdrantClient().Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Wait:           &waitUpsert,
		Points:         points,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert points: %v", err)
	}

	result := fmt.Sprintf("Successfully upserted\nOperation ID: %d\nStatus: %s", upsertResp.OperationId, upsertResp.Status)

	return mcp.NewToolResultText(result), nil
}

func splitIntoChunks(content string, _ string) ([]string, error) {
	const (
		maxTokensPerChunk = 512
		overlapTokens     = 50
		model             = "text-embedding-3-large"
	)

	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get encoding: %v", err)
	}

	tokens := encoding.Encode(content, nil, nil)

	var chunks []string
	var currentChunk []int

	// First pass: collect all chunks without context
	var rawChunks []string
	for i := 0; i < len(tokens); i++ {
		currentChunk = append(currentChunk, tokens[i])

		if len(currentChunk) >= maxTokensPerChunk {
			chunkText := encoding.Decode(currentChunk)
			rawChunks = append(rawChunks, chunkText)

			if len(currentChunk) > overlapTokens {
				currentChunk = currentChunk[len(currentChunk)-overlapTokens:]
			} else {
				currentChunk = []int{}
			}
		}
	}

	// Handle remaining tokens
	if len(currentChunk) > 0 {
		chunkText := encoding.Decode(currentChunk)
		rawChunks = append(rawChunks, chunkText)
	}

	// If there's only one chunk, return it without context
	if len(rawChunks) == 1 {
		return rawChunks, nil
	}

	// If there are multiple chunks, add context to each
	for _, chunkText := range rawChunks {
		contextualizedChunk, err := generateContext(content, chunkText)
		if err != nil {
			return nil, fmt.Errorf("failed to generate context: %v", err)
		}
		chunks = append(chunks, contextualizedChunk)
	}

	return chunks, nil
}

func generateContext(fullText, chunkText string) (string, error) {
	prompt := fmt.Sprintf(`
<document>%s</document>
Here is the chunk we want to situate within the whole document:
<chunk>%s</chunk>
Please give a short succinct context to situate this chunk within the overall document for the purposes of improving search retrieval of the chunk. Answer only with the succinct context and nothing else.
	`, fullText, chunkText)

	// Use codesmart model instead of GPT
	model := "codesmart"

	resp, err := services.DefaultOpenAIClient().CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to generate context: %v", err)
	}

	context := resp.Choices[0].Message.Content
	return fmt.Sprintf("Context: \n%s;\n\nChunk: \n%s", context, chunkText), nil
}

// Update vectorSearchHandler to use codesmart.embedding by default
func vectorSearchHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	collection := arguments["collection"].(string)
	query := arguments["query"].(string)

	ctx := context.Background()

	// Check if collection exists and get info
	collectionInfo, err := qdrantClient().GetCollectionInfo(ctx, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %v", err)
	}

	// Always default to codesmart.embedding
	modelStr := "codesmart.embedding"
	if modelArg, ok := arguments["model"].(string); ok && modelArg != "" {
		embModel, _, err := validateEmbeddingModel(modelArg)
		if err != nil {
			return nil, err
		}
		modelStr = string(embModel)
	}

	// Generate embedding for the query using selected model
	resp, err := services.DefaultOpenAIClient().CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
		Input: []string{query},
		Model: openai.EmbeddingModel(modelStr),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings for query: %v", err)
	}

	// Lower score threshold and add limit
	scoreThreshold := float32(0.3) // Lower threshold to get more results
	limit := uint64(10)            // Limit results to 10

	// Search Qdrant with debug info
	searchResult, err := qdrantClient().Query(ctx, &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQuery(resp.Data[0].Embedding...), // Use Query instead of Vector
		Limit:          &limit,
		ScoreThreshold: &scoreThreshold,
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search in Qdrant: %v", err)
	}

	// Add debug info to results
	var resultText string
	resultText = fmt.Sprintf("Search Results for Collection: %s\nTotal points in collection: %d\nQuery: %s\nModel: %s\nScore threshold: %f\n\n",
		collection,
		collectionInfo.PointsCount,
		query,
		modelStr,
		scoreThreshold)

	if len(searchResult) == 0 {
		resultText += "No results found that match the query with the current threshold.\n"
	}

	for i, hit := range searchResult {
		content := hit.Payload["content"].GetStringValue()
		filePath := hit.Payload["filePath"].GetStringValue()
		usedModel := hit.Payload["model"].GetStringValue()
		resultText += fmt.Sprintf("Result %d (Score: %.4f):\nModel: %s\nFilePath: %s\nContent: %s\n\n",
			i+1, hit.Score, usedModel, filePath, content)
	}

	return mcp.NewToolResultText(resultText), nil
}
