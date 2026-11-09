export namespace desktop {
	
	export class SubmissionDTO {
	    contentType: string;
	    content: string;
	    checksum: string;
	    submittedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SubmissionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contentType = source["contentType"];
	        this.content = source["content"];
	        this.checksum = source["checksum"];
	        this.submittedAt = source["submittedAt"];
	    }
	}
	export class CriterionDTO {
	    key: string;
	    name: string;
	    description: string;
	    weight: number;
	    mandatory: boolean;
	    levels: string[];
	
	    static createFrom(source: any = {}) {
	        return new CriterionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.weight = source["weight"];
	        this.mandatory = source["mandatory"];
	        this.levels = source["levels"];
	    }
	}
	export class StatementDTO {
	    scenario: string;
	    functionalRequirements: string[];
	    nonFunctionalRequirements: string[];
	    constraints: string[];
	    objectives: string[];
	
	    static createFrom(source: any = {}) {
	        return new StatementDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scenario = source["scenario"];
	        this.functionalRequirements = source["functionalRequirements"];
	        this.nonFunctionalRequirements = source["nonFunctionalRequirements"];
	        this.constraints = source["constraints"];
	        this.objectives = source["objectives"];
	    }
	}
	export class AttemptDTO {
	    id: string;
	    challengeId: string;
	    challengeTitle: string;
	    challengeTopic: string;
	    sessionExerciseId: string;
	    state: string;
	    startedAt: string;
	    deadline: string;
	    submittedAt: string;
	    submitReason: string;
	    statement: StatementDTO;
	    criteria: CriterionDTO[];
	    submission?: SubmissionDTO;
	
	    static createFrom(source: any = {}) {
	        return new AttemptDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.challengeId = source["challengeId"];
	        this.challengeTitle = source["challengeTitle"];
	        this.challengeTopic = source["challengeTopic"];
	        this.sessionExerciseId = source["sessionExerciseId"];
	        this.state = source["state"];
	        this.startedAt = source["startedAt"];
	        this.deadline = source["deadline"];
	        this.submittedAt = source["submittedAt"];
	        this.submitReason = source["submitReason"];
	        this.statement = this.convertValues(source["statement"], StatementDTO);
	        this.criteria = this.convertValues(source["criteria"], CriterionDTO);
	        this.submission = this.convertValues(source["submission"], SubmissionDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AttemptHistoryItemDTO {
	    attemptId: string;
	    finishedAt: string;
	    diagramScore: number;
	    understandingScore: number;
	    outcome: string;
	
	    static createFrom(source: any = {}) {
	        return new AttemptHistoryItemDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attemptId = source["attemptId"];
	        this.finishedAt = source["finishedAt"];
	        this.diagramScore = source["diagramScore"];
	        this.understandingScore = source["understandingScore"];
	        this.outcome = source["outcome"];
	    }
	}
	export class DefenseTurnDTO {
	    position: number;
	    question: string;
	    answer: string;
	    askedAt: string;
	    answeredAt: string;
	
	    static createFrom(source: any = {}) {
	        return new DefenseTurnDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.position = source["position"];
	        this.question = source["question"];
	        this.answer = source["answer"];
	        this.askedAt = source["askedAt"];
	        this.answeredAt = source["answeredAt"];
	    }
	}
	export class EvaluationResultDTO {
	    criterionKey: string;
	    criterionName: string;
	    weight: number;
	    level: number;
	    justification: string;
	    evidence: string;
	    insufficientEvidence: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EvaluationResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.criterionKey = source["criterionKey"];
	        this.criterionName = source["criterionName"];
	        this.weight = source["weight"];
	        this.level = source["level"];
	        this.justification = source["justification"];
	        this.evidence = source["evidence"];
	        this.insufficientEvidence = source["insufficientEvidence"];
	    }
	}
	export class EvaluationDTO {
	    attemptId: string;
	    model: string;
	    score: number;
	    summary: string;
	    strengths: string[];
	    gaps: string[];
	    recommendations: string[];
	    results: EvaluationResultDTO[];
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new EvaluationDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attemptId = source["attemptId"];
	        this.model = source["model"];
	        this.score = source["score"];
	        this.summary = source["summary"];
	        this.strengths = source["strengths"];
	        this.gaps = source["gaps"];
	        this.recommendations = source["recommendations"];
	        this.results = this.convertValues(source["results"], EvaluationResultDTO);
	        this.createdAt = source["createdAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OutcomeReasonDTO {
	    kind: string;
	    evaluation: string;
	    criterionKey: string;
	    criterionName: string;
	
	    static createFrom(source: any = {}) {
	        return new OutcomeReasonDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.evaluation = source["evaluation"];
	        this.criterionKey = source["criterionKey"];
	        this.criterionName = source["criterionName"];
	    }
	}
	export class OutcomeDTO {
	    result: string;
	    passScore: number;
	    diagramScore: number;
	    understandingScore: number;
	    reasons: OutcomeReasonDTO[];
	
	    static createFrom(source: any = {}) {
	        return new OutcomeDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.result = source["result"];
	        this.passScore = source["passScore"];
	        this.diagramScore = source["diagramScore"];
	        this.understandingScore = source["understandingScore"];
	        this.reasons = this.convertValues(source["reasons"], OutcomeReasonDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ResultChallengeDTO {
	    id: string;
	    title: string;
	    topic: string;
	    difficulty: string;
	    statement: StatementDTO;
	
	    static createFrom(source: any = {}) {
	        return new ResultChallengeDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.topic = source["topic"];
	        this.difficulty = source["difficulty"];
	        this.statement = this.convertValues(source["statement"], StatementDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AttemptResultDTO {
	    challenge: ResultChallengeDTO;
	    attemptId: string;
	    startedAt: string;
	    submittedAt: string;
	    finishedAt: string;
	    timeSpentSeconds: number;
	    submitReason: string;
	    hintsUsed: number;
	    maxHints: number;
	    outcome: OutcomeDTO;
	    diagram: EvaluationDTO;
	    understanding: EvaluationDTO;
	    defense: DefenseTurnDTO[];
	    submission: string;
	
	    static createFrom(source: any = {}) {
	        return new AttemptResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.challenge = this.convertValues(source["challenge"], ResultChallengeDTO);
	        this.attemptId = source["attemptId"];
	        this.startedAt = source["startedAt"];
	        this.submittedAt = source["submittedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.timeSpentSeconds = source["timeSpentSeconds"];
	        this.submitReason = source["submitReason"];
	        this.hintsUsed = source["hintsUsed"];
	        this.maxHints = source["maxHints"];
	        this.outcome = this.convertValues(source["outcome"], OutcomeDTO);
	        this.diagram = this.convertValues(source["diagram"], EvaluationDTO);
	        this.understanding = this.convertValues(source["understanding"], EvaluationDTO);
	        this.defense = this.convertValues(source["defense"], DefenseTurnDTO);
	        this.submission = source["submission"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CatalogFilterDTO {
	    query: string;
	    status: string;
	    difficulty: string;
	    topic: string;
	
	    static createFrom(source: any = {}) {
	        return new CatalogFilterDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.status = source["status"];
	        this.difficulty = source["difficulty"];
	        this.topic = source["topic"];
	    }
	}
	export class ChallengeCatalogItemDTO {
	    id: string;
	    title: string;
	    topic: string;
	    difficulty: string;
	    timeLimitMinutes: number;
	    status: string;
	    attempts: number;
	    bestDiagramScore: number;
	
	    static createFrom(source: any = {}) {
	        return new ChallengeCatalogItemDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.topic = source["topic"];
	        this.difficulty = source["difficulty"];
	        this.timeLimitMinutes = source["timeLimitMinutes"];
	        this.status = source["status"];
	        this.attempts = source["attempts"];
	        this.bestDiagramScore = source["bestDiagramScore"];
	    }
	}
	export class ChallengeCatalogDTO {
	    total: number;
	    completed: number;
	    inProgress: number;
	    items: ChallengeCatalogItemDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ChallengeCatalogDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.completed = source["completed"];
	        this.inProgress = source["inProgress"];
	        this.items = this.convertValues(source["items"], ChallengeCatalogItemDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ChallengeSummaryDTO {
	    id: string;
	    title: string;
	    topic: string;
	    difficulty: string;
	    timeLimitMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new ChallengeSummaryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.topic = source["topic"];
	        this.difficulty = source["difficulty"];
	        this.timeLimitMinutes = source["timeLimitMinutes"];
	    }
	}
	export class ChunkLoadIssueResult {
	    chunkId: string;
	    itemId: string;
	    filePath: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ChunkLoadIssueResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chunkId = source["chunkId"];
	        this.itemId = source["itemId"];
	        this.filePath = source["filePath"];
	        this.reason = source["reason"];
	    }
	}
	export class ChunkPreviewResult {
	    heading: string;
	    content: string;
	    charCount: number;
	    estimatedTokens: number;
	
	    static createFrom(source: any = {}) {
	        return new ChunkPreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.heading = source["heading"];
	        this.content = source["content"];
	        this.charCount = source["charCount"];
	        this.estimatedTokens = source["estimatedTokens"];
	    }
	}
	
	export class DefenseDTO {
	    total: number;
	    turns: DefenseTurnDTO[];
	    complete: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DefenseDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.turns = this.convertValues(source["turns"], DefenseTurnDTO);
	        this.complete = source["complete"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DefenseQuestionDTO {
	    position: number;
	    question: string;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new DefenseQuestionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.position = source["position"];
	        this.question = source["question"];
	        this.total = source["total"];
	    }
	}
	
	
	
	export class ExerciseOfferDTO {
	    id: string;
	    sessionId: string;
	    messageId: string;
	
	    static createFrom(source: any = {}) {
	        return new ExerciseOfferDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.messageId = source["messageId"];
	    }
	}
	export class FailedSectionResult {
	    heading: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new FailedSectionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.heading = source["heading"];
	        this.reason = source["reason"];
	    }
	}
	export class FolderResult {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new FolderResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class FormattingStatsResult {
	    headingsBefore: number;
	    headingsAfter: number;
	    paragraphsBefore: number;
	    paragraphsAfter: number;
	
	    static createFrom(source: any = {}) {
	        return new FormattingStatsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.headingsBefore = source["headingsBefore"];
	        this.headingsAfter = source["headingsAfter"];
	        this.paragraphsBefore = source["paragraphsBefore"];
	        this.paragraphsAfter = source["paragraphsAfter"];
	    }
	}
	export class HintDTO {
	    id: string;
	    attemptId: string;
	    position: number;
	    content: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new HintDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.attemptId = source["attemptId"];
	        this.position = source["position"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class StructureWarningResult {
	    heading: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new StructureWarningResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.heading = source["heading"];
	        this.message = source["message"];
	    }
	}
	export class ImportPreviewResult {
	    displayPath: string;
	    sourceUrl: string;
	    originalContent: string;
	    formattedContent: string;
	    formattingFailed: boolean;
	    failedSections: FailedSectionResult[];
	    structureWarnings: StructureWarningResult[];
	    stats: FormattingStatsResult;
	    chunks: ChunkPreviewResult[];
	    skipped: boolean;
	    skippedReason: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportPreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.displayPath = source["displayPath"];
	        this.sourceUrl = source["sourceUrl"];
	        this.originalContent = source["originalContent"];
	        this.formattedContent = source["formattedContent"];
	        this.formattingFailed = source["formattingFailed"];
	        this.failedSections = this.convertValues(source["failedSections"], FailedSectionResult);
	        this.structureWarnings = this.convertValues(source["structureWarnings"], StructureWarningResult);
	        this.stats = this.convertValues(source["stats"], FormattingStatsResult);
	        this.chunks = this.convertValues(source["chunks"], ChunkPreviewResult);
	        this.skipped = source["skipped"];
	        this.skippedReason = source["skippedReason"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class IndexStatusResult {
	    state: string;
	    hasSnapshot: boolean;
	    issues: ChunkLoadIssueResult[];
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new IndexStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.hasSnapshot = source["hasSnapshot"];
	        this.issues = this.convertValues(source["issues"], ChunkLoadIssueResult);
	        this.lastError = source["lastError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelChoices {
	    study: string;
	    conversationSummary: string;
	    queryEnrichment: string;
	    noteFormatting: string;
	    architectureLab: string;
	    embedding: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelChoices(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.study = source["study"];
	        this.conversationSummary = source["conversationSummary"];
	        this.queryEnrichment = source["queryEnrichment"];
	        this.noteFormatting = source["noteFormatting"];
	        this.architectureLab = source["architectureLab"];
	        this.embedding = source["embedding"];
	    }
	}
	export class ModelSettingsResult {
	    current: ModelChoices;
	    defaults: ModelChoices;
	
	    static createFrom(source: any = {}) {
	        return new ModelSettingsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = this.convertValues(source["current"], ModelChoices);
	        this.defaults = this.convertValues(source["defaults"], ModelChoices);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class SaveModelSettingsResult {
	    embeddingModelChanged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveModelSettingsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.embeddingModelChanged = source["embeddingModelChanged"];
	    }
	}
	export class SelectableModelsResult {
	    chat: string[];
	    embedding: string[];
	
	    static createFrom(source: any = {}) {
	        return new SelectableModelsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chat = source["chat"];
	        this.embedding = source["embedding"];
	    }
	}
	export class SessionExerciseDTO {
	    challengeId: string;
	    title: string;
	    topic: string;
	    difficulty: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionExerciseDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.challengeId = source["challengeId"];
	        this.title = source["title"];
	        this.topic = source["topic"];
	        this.difficulty = source["difficulty"];
	    }
	}
	export class SessionOptionDTO {
	    id: string;
	    topic: string;
	    folderName: string;
	    startedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionOptionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.topic = source["topic"];
	        this.folderName = source["folderName"];
	        this.startedAt = source["startedAt"];
	    }
	}
	export class SessionSourceResult {
	    itemId: string;
	    title: string;
	    path: string;
	    chunkCount: number;
	    ingestedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionSourceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.title = source["title"];
	        this.path = source["path"];
	        this.chunkCount = source["chunkCount"];
	        this.ingestedAt = source["ingestedAt"];
	    }
	}
	export class SourceDocumentSegmentResult {
	    text: string;
	    chunkId: string;
	
	    static createFrom(source: any = {}) {
	        return new SourceDocumentSegmentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.chunkId = source["chunkId"];
	    }
	}
	export class SourceDocumentResult {
	    itemId: string;
	    title: string;
	    path: string;
	    segments: SourceDocumentSegmentResult[];
	
	    static createFrom(source: any = {}) {
	        return new SourceDocumentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.title = source["title"];
	        this.path = source["path"];
	        this.segments = this.convertValues(source["segments"], SourceDocumentSegmentResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class StudyContextResult {
	    state: string;
	    model: string;
	    usedTokens: number;
	    contextLength: number;
	    estimated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StudyContextResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.model = source["model"];
	        this.usedTokens = source["usedTokens"];
	        this.contextLength = source["contextLength"];
	        this.estimated = source["estimated"];
	    }
	}
	export class StudySourceResult {
	    chunkId: string;
	    itemId: string;
	    sourceType: string;
	    filePath: string;
	    heading: string;
	    concept: string;
	    score: number;
	    excerpt: string;
	
	    static createFrom(source: any = {}) {
	        return new StudySourceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chunkId = source["chunkId"];
	        this.itemId = source["itemId"];
	        this.sourceType = source["sourceType"];
	        this.filePath = source["filePath"];
	        this.heading = source["heading"];
	        this.concept = source["concept"];
	        this.score = source["score"];
	        this.excerpt = source["excerpt"];
	    }
	}
	export class StudyMessageResult {
	    id: string;
	    role: string;
	    content: string;
	    createdAt: string;
	    sources: StudySourceResult[];
	
	    static createFrom(source: any = {}) {
	        return new StudyMessageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	        this.sources = this.convertValues(source["sources"], StudySourceResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StudyMessagePageResult {
	    messages: StudyMessageResult[];
	    hasMore: boolean;
	    nextCursor: string;
	
	    static createFrom(source: any = {}) {
	        return new StudyMessagePageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], StudyMessageResult);
	        this.hasMore = source["hasMore"];
	        this.nextCursor = source["nextCursor"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class StudySessionResult {
	    id: string;
	    topic: string;
	    folderId: string;
	    goal: string;
	    startedAt: string;
	    context: StudyContextResult;
	
	    static createFrom(source: any = {}) {
	        return new StudySessionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.topic = source["topic"];
	        this.folderId = source["folderId"];
	        this.goal = source["goal"];
	        this.startedAt = source["startedAt"];
	        this.context = this.convertValues(source["context"], StudyContextResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StudySessionHistoryResult {
	    session: StudySessionResult;
	    messages: StudyMessageResult[];
	    hasMore: boolean;
	    nextCursor: string;
	    summaryCoversUntilMessageId: string;
	
	    static createFrom(source: any = {}) {
	        return new StudySessionHistoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session = this.convertValues(source["session"], StudySessionResult);
	        this.messages = this.convertValues(source["messages"], StudyMessageResult);
	        this.hasMore = source["hasMore"];
	        this.nextCursor = source["nextCursor"];
	        this.summaryCoversUntilMessageId = source["summaryCoversUntilMessageId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class StudySessionUsageResult {
	    inputTokens: number;
	    outputTokens: number;
	    cost: number;
	    unpricedCalls: number;
	    calls: number;
	    context: StudyContextResult;
	
	    static createFrom(source: any = {}) {
	        return new StudySessionUsageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cost = source["cost"];
	        this.unpricedCalls = source["unpricedCalls"];
	        this.calls = source["calls"];
	        this.context = this.convertValues(source["context"], StudyContextResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class StudySummaryResult {
	    found: boolean;
	    content: string;
	    coversUntilMessageId: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StudySummaryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.content = source["content"];
	        this.coversUntilMessageId = source["coversUntilMessageId"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	
	export class UserProfileInput {
	    name: string;
	    assistantName: string;
	    area: string;
	    experienceLevel: string;
	    studyStyle: string;
	    assistantLanguage: string;
	
	    static createFrom(source: any = {}) {
	        return new UserProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.assistantName = source["assistantName"];
	        this.area = source["area"];
	        this.experienceLevel = source["experienceLevel"];
	        this.studyStyle = source["studyStyle"];
	        this.assistantLanguage = source["assistantLanguage"];
	    }
	}

}

