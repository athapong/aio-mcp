package processors

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/athapong/aio-mcp/pkg/graph"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/jdkato/prose/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	processingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "nlp_processing_duration_seconds",
			Help: "Time spent processing documents",
		},
		[]string{"processor_type"},
	)

	entityCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nlp_entities_extracted_total",
			Help: "Number of entities extracted",
		},
		[]string{"entity_type"},
	)
)

const (
	EntityTypeTechnology  = "TECHNOLOGY"
	EntityTypeFramework   = "FRAMEWORK"
	EntityTypeLanguage    = "LANGUAGE"
	EntityTypeAPI         = "API"
	EntityTypeDatabase    = "DATABASE"
	EntityTypeArchPattern = "ARCH_PATTERN"
	EntityTypeFinProduct  = "FINANCIAL_PRODUCT"
	EntityTypeTransaction = "TRANSACTION"
	EntityTypeCurrency    = "CURRENCY"
	EntityTypeAccount     = "ACCOUNT"
	EntityTypeRegulation  = "REGULATION"
	EntityTypeRisk        = "RISK"

	// Technical Entity Types
	EntityTypeComponent     = "COMPONENT"
	EntityTypeService       = "SERVICE"
	EntityTypeLibrary       = "LIBRARY"
	EntityTypeProtocol      = "PROTOCOL"
	EntityTypeCloud         = "CLOUD"
	EntityTypeDevOps        = "DEVOPS"
	EntityTypeDesignPattern = "DESIGN_PATTERN"
	EntityTypeSecurity      = "SECURITY"
	EntityTypeInfra         = "INFRASTRUCTURE"
	EntityTypeML            = "MACHINE_LEARNING"
	EntityTypeTest          = "TESTING"
	EntityTypeMonitoring    = "MONITORING"

	// Relation Types
	RelationDependsOn    = "DEPENDS_ON"
	RelationImplements   = "IMPLEMENTS"
	RelationCommunicates = "COMMUNICATES_WITH"
	RelationExtends      = "EXTENDS"
	RelationConfigures   = "CONFIGURES"
	RelationDeploys      = "DEPLOYS"
	RelationMonitors     = "MONITORS"
	RelationTests        = "TESTS"
	RelationIntegrates   = "INTEGRATES"
	RelationOrchestrates = "ORCHESTRATES"
)

func init() {
	prometheus.MustRegister(processingDuration)
	prometheus.MustRegister(entityCount)
}

// NLPProcessor implements basic NLP processing using prose
type NLPProcessor struct {
	logger *logrus.Logger
}

// NewNLPProcessor creates a new NLP processor
func NewNLPProcessor() *NLPProcessor {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &NLPProcessor{
		logger: logger,
	}
}

// Process implements the DocumentProcessor interface
func (p *NLPProcessor) Process(ctx context.Context, content []byte, metadata map[string]interface{}) (*graph.Document, error) {
	timer := prometheus.NewTimer(processingDuration.WithLabelValues("nlp"))
	defer timer.ObserveDuration()

	p.logger.WithField("content_length", len(content)).Info("Starting NLP processing")

	// Create prose document
	doc, err := prose.NewDocument(string(content))
	if err != nil {
		p.logger.WithError(err).Error("Failed to create prose document")
		return nil, err
	}

	// Process entities and relations
	entities, relations := p.extractEntitiesAndRelations(doc)

	// Perform coreference resolution
	entities = p.resolveCoreferenceChains(entities, doc)

	// Extract keywords using TextRank
	keywords := p.extractKeywords(doc)

	// Create processed document
	processed := &graph.Document{
		Content:     string(content),
		Entities:    entities,
		Relations:   relations,
		Keywords:    keywords,
		Metadata:    metadata,
		ProcessedAt: time.Now(),
	}

	p.logger.WithFields(logrus.Fields{
		"entities_count":  len(entities),
		"relations_count": len(relations),
		"keywords_count":  len(keywords),
	}).Info("NLP processing completed")

	return processed, nil
}

func (p *NLPProcessor) extractEntitiesAndRelations(doc *prose.Document) ([]graph.Entity, []graph.Relationship) {
	entities := make([]graph.Entity, 0)
	relations := make([]graph.Relationship, 0)

	tokens := doc.Tokens()
	tokensText := make([]string, len(tokens))
	for i, tok := range tokens {
		tokensText[i] = tok.Text
	}

	// Combined tech and banking patterns
	entityPatterns := map[string]string{
		// Existing tech patterns
		`(?i)(kubernetes|docker|jenkins|git|terraform|aws|azure)`: EntityTypeTechnology,
		`(?i)(spring|react|angular|vue|django|flask)`:             EntityTypeFramework,
		`(?i)(java|python|golang|javascript|typescript)`:          EntityTypeLanguage,
		`(?i)(rest|graphql|grpc|soap|websocket)`:                  EntityTypeAPI,
		`(?i)(mysql|postgresql|mongodb|redis|elasticsearch)`:      EntityTypeDatabase,
		`(?i)(microservices|mvc|mvvm|cqrs|event-sourcing)`:        EntityTypeArchPattern,

		// New banking patterns
		`(?i)(loan|mortgage|deposit|credit card|debit card|savings account)`: EntityTypeFinProduct,
		`(?i)(payment|transfer|withdrawal|deposit|transaction)`:              EntityTypeTransaction,
		`(?i)(usd|eur|gbp|jpy|thb|sgd|\$|€|£|¥)`:                             EntityTypeCurrency,
		`(?i)(checking|savings|current|investment|retirement)`:               EntityTypeAccount,
		`(?i)(basel|kyc|aml|fatca|gdpr|psd2)`:                                EntityTypeRegulation,
		`(?i)(credit risk|market risk|operational risk|liquidity risk)`:      EntityTypeRisk,

		// Enhanced IT-specific patterns
		// Components and Services
		`(?i)(microservice|api gateway|load balancer|cache|queue)`: EntityTypeComponent,
		`(?i)(rest api|graphql|grpc|webhook|service mesh)`:         EntityTypeService,

		// Frameworks and Libraries
		`(?i)(spring|react|angular|vue|django|flask|express)`:     EntityTypeFramework,
		`(?i)(numpy|pandas|tensorflow|pytorch|kubernetes|docker)`: EntityTypeLibrary,

		// Protocols and Standards
		`(?i)(http[s]?|tcp|udp|mqtt|amqp|websocket)`: EntityTypeProtocol,
		`(?i)(oauth|jwt|saml|openid|x509)`:           EntityTypeSecurity,

		// Databases and Storage
		`(?i)(mysql|postgresql|mongodb|redis|elasticsearch|kafka)`: EntityTypeDatabase,

		// Cloud and Infrastructure
		`(?i)(aws|azure|gcp|cloud|kubernetes|docker)`: EntityTypeCloud,
		`(?i)(jenkins|gitlab|github|circleci|argocd)`: EntityTypeDevOps,

		// Architecture Patterns
		`(?i)(microservices|event-driven|cqrs|saga|circuit breaker)`: EntityTypeArchPattern,
		`(?i)(singleton|factory|observer|strategy|decorator)`:        EntityTypeDesignPattern,

		// ML and Analytics
		`(?i)(tensorflow|pytorch|scikit-learn|bert|gpt|transformers)`: EntityTypeML,

		// Languages and Tools
		`(?i)(java|python|golang|javascript|typescript|rust)`: EntityTypeLanguage,
		`(?i)(junit|pytest|jest|selenium|cypress)`:            EntityTypeTest,
		`(?i)(prometheus|grafana|datadog|newrelic|splunk)`:    EntityTypeMonitoring,
	}

	text := doc.Text
	for pattern, entityType := range entityPatterns {
		matches := regexp.MustCompile(pattern).FindAllStringIndex(text, -1)
		for _, match := range matches {
			entity := graph.Entity{
				Label: text[match[0]:match[1]],
				Type:  entityType,
				Properties: map[string]interface{}{
					"start_pos": match[0],
					"end_pos":   match[1],
				},
				Confidence: 0.9,
			}
			entities = append(entities, entity)
			entityCount.WithLabelValues(entityType).Inc()
		}
	}

	// Extract tech-specific relationships
	for i, tok := range tokens {
		if tok.Tag == "VB" || tok.Tag == "VBZ" || tok.Tag == "VBP" {
			if p.isTechnicalVerb(tok.Text) || p.isBankingVerb(tok.Text) {
				subj := p.findNearestEntity(tokens, i, -1)
				obj := p.findNearestEntity(tokens, i, 1)

				if subj != nil && obj != nil {
					relType := p.getTechRelationType(tok.Text)
					if relType == "RELATED_TO" {
						relType = p.getBankingRelationType(tok.Text)
					}

					rel := graph.Relationship{
						Type:       relType,
						From:       subj.Text,
						To:         obj.Text,
						Confidence: 0.85,
					}
					relations = append(relations, rel)
				}
			}
		}
	}
	// Enhanced relation extraction
	techRelations := map[string]string{
		"depends":      RelationDependsOn,
		"implements":   RelationImplements,
		"calls":        RelationCommunicates,
		"extends":      RelationExtends,
		"configures":   RelationConfigures,
		"deploys":      RelationDeploys,
		"monitors":     RelationMonitors,
		"tests":        RelationTests,
		"integrates":   RelationIntegrates,
		"orchestrates": RelationOrchestrates,
	}

	// Enhanced relation extraction using techRelations
	for i, tok := range tokens {
		if tok.Tag == "VB" || tok.Tag == "VBZ" || tok.Tag == "VBP" {
			if relType, exists := techRelations[strings.ToLower(tok.Text)]; exists {
				subj := p.findNearestEntity(tokens, i, -1)
				obj := p.findNearestEntity(tokens, i, 1)

				if subj != nil && obj != nil {
					rel := graph.Relationship{
						Type:       relType,
						From:       subj.Text,
						To:         obj.Text,
						Confidence: 0.85,
					}
					relations = append(relations, rel)
				}
			}
		}
	}

	return entities, relations
}

func (p *NLPProcessor) resolveCoreferenceChains(entities []graph.Entity, doc *prose.Document) []graph.Entity {
	// Simple pronoun resolution
	pronounIndices := mapset.NewSet[int]()
	entityMap := make(map[string]*graph.Entity)
	sentences := doc.Sentences()

	// First pass: collect entities and pronouns
	for i := range entities {
		if p.isPronoun(entities[i].Label) {
			pronounIndices.Add(i)
		} else {
			entityMap[entities[i].Label] = &entities[i]
		}
	}

	// Second pass: resolve pronouns
	resolved := make([]graph.Entity, len(entities))
	copy(resolved, entities)

	for _, sentIdx := range pronounIndices.ToSlice() {
		pronoun := entities[sentIdx]

		// Find the sentence containing this pronoun
		containingSentence := -1
		pronounPos := -1
		for i, sent := range sentences {
			if strings.Contains(sent.Text, pronoun.Label) {
				containingSentence = i
				pronounPos = strings.Index(sent.Text, pronoun.Label)
				break
			}
		}

		if containingSentence < 0 {
			continue
		}

		// Look for the nearest matching entity in previous sentences
		var bestMatch *graph.Entity
		bestDistance := float64(1000000)

		for i := containingSentence; i >= 0 && i >= containingSentence-3; i-- {
			for _, ent := range entities {
				if p.isPronoun(ent.Label) {
					continue
				}

				if p.canBeCoreferent(pronoun.Label, ent.Label) {
					distance := float64(pronounPos + (containingSentence-i)*100)
					if distance < bestDistance {
						bestDistance = distance
						bestMatch = &ent
					}
				}
			}
		}

		if bestMatch != nil {
			resolved[sentIdx] = *bestMatch
		}
	}

	return resolved
}

func (p *NLPProcessor) canBeCoreferent(pronoun, entity string) bool {
	pronoun = strings.ToLower(pronoun)

	// Gender and number agreement
	malePronouns := mapset.NewSet[string]("he", "him", "his")
	femalePronouns := mapset.NewSet[string]("she", "her", "hers")
	pluralPronouns := mapset.NewSet[string]("they", "them", "their", "theirs")

	if malePronouns.Contains(pronoun) {
		return p.isMalePerson(entity)
	}
	if femalePronouns.Contains(pronoun) {
		return p.isFemalePerson(entity)
	}
	if pluralPronouns.Contains(pronoun) {
		return p.isPlural(entity)
	}

	return true
}

func (p *NLPProcessor) extractKeywords(doc *prose.Document) []graph.Keyword {
	tokens := doc.Tokens()
	sentences := doc.Sentences()

	// Create graph of word co-occurrences
	graphMap := make(map[string]map[string]float64)
	wordScores := make(map[string]float64)

	// Initialize graph
	for _, tok := range tokens {
		if !p.isStopWord(tok.Text) && tok.Tag[0] == 'N' { // Consider only nouns
			graphMap[tok.Text] = make(map[string]float64)
			wordScores[tok.Text] = 1.0
		}
	}

	// Build co-occurrence graph
	window := 4 // co-occurrence window size
	for _, sent := range sentences {
		words := strings.Fields(sent.Text)
		for i, word := range words {
			if _, exists := graphMap[word]; !exists {
				continue
			}

			// Look at words within window
			start := max(0, i-window)
			end := min(len(words), i+window)

			for j := start; j < end; j++ {
				if i != j {
					coWord := words[j]
					if _, exists := graphMap[coWord]; exists {
						graphMap[word][coWord] += 1.0
						graphMap[coWord][word] += 1.0
					}
				}
			}
		}
	}

	// Run TextRank algorithm
	damping := 0.85
	epsilon := 0.0001
	maxIterations := 50

	for iter := 0; iter < maxIterations; iter++ {
		diff := 0.0
		newScores := make(map[string]float64)

		// Update each word's score
		for word := range graphMap {
			sum := 0.0
			for other, weight := range graphMap[word] {
				sum += weight * wordScores[other] / p.sumEdgeWeights(graphMap[other])
			}
			newScore := (1 - damping) + damping*sum
			diff += abs(newScore - wordScores[word])
			newScores[word] = newScore
		}

		// Check convergence
		if diff < epsilon {
			break
		}
		wordScores = newScores
	}

	// Add technical term boosting
	technicalTermBoost := 1.5
	technicalPatterns := []string{
		"api", "sdk", "rest", "graphql", "cloud",
		"docker", "kubernetes", "microservices",
		"database", "cache", "server", "client",
		"authentication", "security", "deployment",
	}

	// Add banking term boosting
	bankingTermBoost := 1.5
	bankingPatterns := []string{
		"account", "payment", "transfer", "loan", "credit",
		"mortgage", "investment", "risk", "compliance", "regulatory",
		"banking", "financial", "transaction", "balance", "interest",
		"deposit", "withdrawal", "card", "atm", "branch",
	}

	// Enhanced IT-specific keyword boosting
	technicalPatterns = append(technicalPatterns,
		// Architecture and Design
		"microservice", "api", "event-driven", "serverless", "container",
		"kubernetes", "docker", "service-mesh", "cloud-native",

		// Development and DevOps
		"ci/cd", "pipeline", "git", "testing", "deployment", "monitoring",
		"logging", "tracing", "observability", "automation",

		// Security and Compliance
		"authentication", "authorization", "encryption", "security",
		"compliance", "oauth", "jwt", "certificate", "firewall",

		// Data and Storage
		"database", "cache", "queue", "stream", "persistence",
		"replication", "sharding", "backup", "recovery", "migration",

		// Integration and Communication
		"rest", "graphql", "grpc", "websocket", "message-queue",
		"event-bus", "pubsub", "webhook", "protocol", "api-gateway",

		// Performance and Reliability
		"scalability", "availability", "reliability", "performance",
		"latency", "throughput", "failover", "redundancy", "backup",
	)

	for word, score := range wordScores {
		// Existing tech boost
		for _, term := range technicalPatterns {
			if strings.Contains(strings.ToLower(word), term) {
				wordScores[word] = score * technicalTermBoost
				break
			}
		}
		// Additional banking boost
		for _, term := range bankingPatterns {
			if strings.Contains(strings.ToLower(word), term) {
				wordScores[word] = score * bankingTermBoost
				break
			}
		}
	}

	// Convert to keywords
	keywords := make([]graph.Keyword, 0)
	for word, score := range wordScores {
		// Find word position in original text
		startPos := strings.Index(doc.Text, word)
		if startPos >= 0 {
			keyword := graph.Keyword{
				Text:     word,
				Score:    score,
				StartPos: startPos,
				EndPos:   startPos + len(word),
				Type:     "keyword",
			}
			keywords = append(keywords, keyword)
		}
	}

	// Sort keywords by score and take top N
	sort.Slice(keywords, func(i, j int) bool {
		return keywords[i].Score > keywords[j].Score
	})

	maxKeywords := 10
	if len(keywords) > maxKeywords {
		keywords = keywords[:maxKeywords]
	}

	return keywords
}

// SupportedTypes implements the DocumentProcessor interface
func (p *NLPProcessor) SupportedTypes() []string {
	return []string{"text/plain", "text/markdown", "text/html"}
}

func (p *NLPProcessor) isPronoun(word string) bool {
	pronouns := map[string]bool{
		"he": true, "she": true, "it": true, "they": true,
		"him": true, "her": true, "them": true,
		"his": true, "hers": true, "its": true, "their": true,
		"this": true, "that": true, "these": true, "those": true,
	}

	return pronouns[strings.ToLower(word)]
}

func (p *NLPProcessor) isStopWord(word string) bool {
	stopWords := mapset.NewSet[string]("the", "a", "an", "and", "or", "but", "in", "on", "at", "to", "for", "of", "with", "by")
	return stopWords.Contains(strings.ToLower(word))
}

func (p *NLPProcessor) sumEdgeWeights(edges map[string]float64) float64 {
	sum := 0.0
	for _, weight := range edges {
		sum += weight
	}
	return sum
}

func (p *NLPProcessor) isMalePerson(entity string) bool {
	maleIndicators := []string{"Mr.", "Mr", "he", "him", "his", "father", "brother", "son"}
	for _, indicator := range maleIndicators {
		if strings.Contains(entity, indicator) {
			return true
		}
	}
	return false
}

func (p *NLPProcessor) isFemalePerson(entity string) bool {
	femaleIndicators := []string{"Mrs.", "Mrs", "Ms.", "Ms", "she", "her", "mother", "sister", "daughter"}
	for _, indicator := range femaleIndicators {
		if strings.Contains(entity, indicator) {
			return true
		}
	}
	return false
}

func (p *NLPProcessor) isPlural(entity string) bool {
	// Simple check for plural forms
	return strings.HasSuffix(entity, "s") ||
		strings.HasSuffix(entity, "ren") || // children
		strings.HasSuffix(entity, "ple") || // people
		strings.Contains(entity, " and ")
}

func (p *NLPProcessor) isTechnicalVerb(verb string) bool {
	technicalVerbs := mapset.NewSet[string](
		"deploys", "implements", "integrates", "connects",
		"hosts", "serves", "queries", "processes",
		"executes", "compiles", "builds", "tests",
	)
	return technicalVerbs.Contains(strings.ToLower(verb))
}

func (p *NLPProcessor) getTechRelationType(verb string) string {
	relationMap := map[string]string{
		"deploys":    "DEPLOYS_TO",
		"implements": "IMPLEMENTS",
		"integrates": "INTEGRATES_WITH",
		"connects":   "CONNECTS_TO",
		"hosts":      "HOSTS",
		"serves":     "SERVES",
		"queries":    "QUERIES",
		"processes":  "PROCESSES",
		"executes":   "EXECUTES",
		"compiles":   "COMPILES",
		"builds":     "BUILDS",
		"tests":      "TESTS",
	}

	if relType, exists := relationMap[strings.ToLower(verb)]; exists {
		return relType
	}
	return "RELATED_TO"
}

func (p *NLPProcessor) findNearestEntity(tokens []prose.Token, pos int, direction int) *prose.Token {
	maxDistance := 5
	patterns := []string{
		// Existing tech patterns
		"API", "SDK", "CLI", "GUI", "REST", "HTTP",
		"Database", "Server", "Client", "Service",
		"Container", "Cloud", "Cluster", "Pod",

		// Banking patterns
		"Account", "Payment", "Transfer", "Loan",
		"Card", "Balance", "Transaction", "Investment",
		"Risk", "Compliance", "Bank", "Branch",
	}

	start := pos + direction
	var end int
	if direction > 0 {
		end = min(pos+maxDistance, len(tokens))
	} else {
		end = max(pos-maxDistance, 0)
	}

	for i := start; direction > 0 && i < end || direction < 0 && i >= end; i += direction {
		token := tokens[i]
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(token.Text), strings.ToLower(pattern)) {
				return &tokens[i]
			}
		}
	}

	return nil
}

func (p *NLPProcessor) isBankingVerb(verb string) bool {
	bankingVerbs := mapset.NewSet[string](
		"transfers", "deposits", "withdraws", "pays",
		"invests", "lends", "borrows", "processes",
		"approves", "declines", "validates", "authorizes",
	)
	return bankingVerbs.Contains(strings.ToLower(verb))
}

func (p *NLPProcessor) getBankingRelationType(verb string) string {
	relationMap := map[string]string{
		"transfers":  "TRANSFERS_TO",
		"deposits":   "DEPOSITS_INTO",
		"withdraws":  "WITHDRAWS_FROM",
		"pays":       "PAYS_TO",
		"invests":    "INVESTS_IN",
		"lends":      "LENDS_TO",
		"borrows":    "BORROWS_FROM",
		"processes":  "PROCESSES",
		"approves":   "APPROVES",
		"declines":   "DECLINES",
		"validates":  "VALIDATES",
		"authorizes": "AUTHORIZES",
	}

	if relType, exists := relationMap[strings.ToLower(verb)]; exists {
		return relType
	}
	return "RELATED_TO"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
