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
	    starterDiagram: string;
	
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
	        this.starterDiagram = source["starterDiagram"];
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
	export class CanonicalHistoryItemDTO {
	    attemptId: string;
	    difficulty: string;
	    finishedAt: string;
	    diagramScore: number;
	    understandingScore: number;
	    outcome: string;
	
	    static createFrom(source: any = {}) {
	        return new CanonicalHistoryItemDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attemptId = source["attemptId"];
	        this.difficulty = source["difficulty"];
	        this.finishedAt = source["finishedAt"];
	        this.diagramScore = source["diagramScore"];
	        this.understandingScore = source["understandingScore"];
	        this.outcome = source["outcome"];
	    }
	}
	export class DifficultyStateDTO {
	    difficulty: string;
	    generated: boolean;
	    challengeId: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new DifficultyStateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.difficulty = source["difficulty"];
	        this.generated = source["generated"];
	        this.challengeId = source["challengeId"];
	        this.status = source["status"];
	    }
	}
	export class CanonicalProblemDTO {
	    id: string;
	    name: string;
	    summary: string;
	    difficulties: DifficultyStateDTO[];
	    attempts: number;
	    hasHistory: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CanonicalProblemDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.summary = source["summary"];
	        this.difficulties = this.convertValues(source["difficulties"], DifficultyStateDTO);
	        this.attempts = source["attempts"];
	        this.hasHistory = source["hasHistory"];
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
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new CatalogFilterDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.status = source["status"];
	        this.difficulty = source["difficulty"];
	        this.topic = source["topic"];
	        this.kind = source["kind"];
	    }
	}
	export class ChallengeCatalogItemDTO {
	    id: string;
	    kind: string;
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
	        this.kind = source["kind"];
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
	    kind: string;
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
	        this.kind = source["kind"];
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
	    archLabEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FolderResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.archLabEnabled = source["archLabEnabled"];
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
	export class LatestResultDTO {
	    outcome: string;
	    diagramScore: number;
	    understandingScore: number;
	
	    static createFrom(source: any = {}) {
	        return new LatestResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outcome = source["outcome"];
	        this.diagramScore = source["diagramScore"];
	        this.understandingScore = source["understandingScore"];
	    }
	}
	export class MindMapDTO {
	    id: string;
	    sessionId: string;
	    sessionTitle: string;
	    title: string;
	    createdAt: string;
	    updatedAt: string;
	    messageId: string;
	    diagramXml: string;
	
	    static createFrom(source: any = {}) {
	        return new MindMapDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.sessionTitle = source["sessionTitle"];
	        this.title = source["title"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.messageId = source["messageId"];
	        this.diagramXml = source["diagramXml"];
	    }
	}
	export class MindMapOfferDTO {
	    id: string;
	    sessionId: string;
	    messageId: string;
	
	    static createFrom(source: any = {}) {
	        return new MindMapOfferDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.messageId = source["messageId"];
	    }
	}
	export class MindMapSummaryDTO {
	    id: string;
	    sessionId: string;
	    sessionTitle: string;
	    title: string;
	    createdAt: string;
	    updatedAt: string;
	    messageId: string;
	
	    static createFrom(source: any = {}) {
	        return new MindMapSummaryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.sessionTitle = source["sessionTitle"];
	        this.title = source["title"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.messageId = source["messageId"];
	    }
	}
	export class ModelChoices {
	    study: string;
	    conversationSummary: string;
	    queryEnrichment: string;
	    noteFormatting: string;
	    architectureLab: string;
	    quiz: string;
	    mindMap: string;
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
	        this.quiz = source["quiz"];
	        this.mindMap = source["mindMap"];
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
	
	
	export class ProgressChallengeResult {
	    challengeId: string;
	    title: string;
	    difficulty: string;
	    timeLimitMinutes: number;
	    diagramScore?: number;
	    defenseScore?: number;
	    result: string;
	    attempts: number;
	
	    static createFrom(source: any = {}) {
	        return new ProgressChallengeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.challengeId = source["challengeId"];
	        this.title = source["title"];
	        this.difficulty = source["difficulty"];
	        this.timeLimitMinutes = source["timeLimitMinutes"];
	        this.diagramScore = source["diagramScore"];
	        this.defenseScore = source["defenseScore"];
	        this.result = source["result"];
	        this.attempts = source["attempts"];
	    }
	}
	export class ProgressCitationResult {
	    kind: string;
	    id: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgressCitationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.label = source["label"];
	    }
	}
	export class ProgressProofResult {
	    assessment: string;
	    description: string;
	    assisted: boolean;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgressProofResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assessment = source["assessment"];
	        this.description = source["description"];
	        this.assisted = source["assisted"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ProgressConceptResult {
	    name: string;
	    evidenceCount: number;
	    proofs: ProgressProofResult[];
	
	    static createFrom(source: any = {}) {
	        return new ProgressConceptResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.evidenceCount = source["evidenceCount"];
	        this.proofs = this.convertValues(source["proofs"], ProgressProofResult);
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
	export class ProgressCountersResult {
	    folders: number;
	    topics: number;
	    topicsToReview: number;
	
	    static createFrom(source: any = {}) {
	        return new ProgressCountersResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folders = source["folders"];
	        this.topics = source["topics"];
	        this.topicsToReview = source["topicsToReview"];
	    }
	}
	export class ProgressTopicResult {
	    sessionId: string;
	    name: string;
	    quizScore?: number;
	    labScore?: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgressTopicResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.name = source["name"];
	        this.quizScore = source["quizScore"];
	        this.labScore = source["labScore"];
	        this.status = source["status"];
	    }
	}
	export class ProgressFolderResult {
	    id: string;
	    name: string;
	    topicCount: number;
	    topics: ProgressTopicResult[];
	
	    static createFrom(source: any = {}) {
	        return new ProgressFolderResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.topicCount = source["topicCount"];
	        this.topics = this.convertValues(source["topics"], ProgressTopicResult);
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
	export class ProgressDashboardResult {
	    counters: ProgressCountersResult;
	    folders: ProgressFolderResult[];
	
	    static createFrom(source: any = {}) {
	        return new ProgressDashboardResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.counters = this.convertValues(source["counters"], ProgressCountersResult);
	        this.folders = this.convertValues(source["folders"], ProgressFolderResult);
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
	export class ProgressDifficultyResult {
	    name: string;
	    source: string;
	    challengeTitle: string;
	    toReview: boolean;
	    evidenceCount: number;
	    proofs: ProgressProofResult[];
	
	    static createFrom(source: any = {}) {
	        return new ProgressDifficultyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.challengeTitle = source["challengeTitle"];
	        this.toReview = source["toReview"];
	        this.evidenceCount = source["evidenceCount"];
	        this.proofs = this.convertValues(source["proofs"], ProgressProofResult);
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
	export class ProgressEvolutionPointResult {
	    order: number;
	    itemNumber: number;
	    title: string;
	    score: number;
	    at: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgressEvolutionPointResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.order = source["order"];
	        this.itemNumber = source["itemNumber"];
	        this.title = source["title"];
	        this.score = source["score"];
	        this.at = source["at"];
	    }
	}
	export class ProgressEvolutionResult {
	    quiz: ProgressEvolutionPointResult[];
	    lab: ProgressEvolutionPointResult[];
	    passScore: number;
	    enough: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProgressEvolutionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.quiz = this.convertValues(source["quiz"], ProgressEvolutionPointResult);
	        this.lab = this.convertValues(source["lab"], ProgressEvolutionPointResult);
	        this.passScore = source["passScore"];
	        this.enough = source["enough"];
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
	
	export class ProgressStatementResult {
	    text: string;
	    citations: ProgressCitationResult[];
	
	    static createFrom(source: any = {}) {
	        return new ProgressStatementResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.citations = this.convertValues(source["citations"], ProgressCitationResult);
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
	export class ProgressInsightResult {
	    analysis: ProgressStatementResult[];
	    recommendations: ProgressStatementResult[];
	    model: string;
	    language: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgressInsightResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.analysis = this.convertValues(source["analysis"], ProgressStatementResult);
	        this.recommendations = this.convertValues(source["recommendations"], ProgressStatementResult);
	        this.model = source["model"];
	        this.language = source["language"];
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
	export class ProgressInsightStateResult {
	    insight?: ProgressInsightResult;
	    stale: boolean;
	    canGenerate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProgressInsightStateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.insight = this.convertValues(source["insight"], ProgressInsightResult);
	        this.stale = source["stale"];
	        this.canGenerate = source["canGenerate"];
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
	
	export class ProgressQuizResult {
	    quizId: string;
	    number: number;
	    difficulty: string;
	    percent?: number;
	    attempts: number;
	
	    static createFrom(source: any = {}) {
	        return new ProgressQuizResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.quizId = source["quizId"];
	        this.number = source["number"];
	        this.difficulty = source["difficulty"];
	        this.percent = source["percent"];
	        this.attempts = source["attempts"];
	    }
	}
	
	export class ProgressTopicDetailResult {
	    sessionId: string;
	    name: string;
	    folderName: string;
	    quizCount: number;
	    challengeCount: number;
	    conceptCount: number;
	    quizzes: ProgressQuizResult[];
	    challenges: ProgressChallengeResult[];
	    understood: ProgressConceptResult[];
	    difficulties: ProgressDifficultyResult[];
	    explainedOnly: ProgressConceptResult[];
	    evolution: ProgressEvolutionResult;
	    insight: ProgressInsightStateResult;
	
	    static createFrom(source: any = {}) {
	        return new ProgressTopicDetailResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.name = source["name"];
	        this.folderName = source["folderName"];
	        this.quizCount = source["quizCount"];
	        this.challengeCount = source["challengeCount"];
	        this.conceptCount = source["conceptCount"];
	        this.quizzes = this.convertValues(source["quizzes"], ProgressQuizResult);
	        this.challenges = this.convertValues(source["challenges"], ProgressChallengeResult);
	        this.understood = this.convertValues(source["understood"], ProgressConceptResult);
	        this.difficulties = this.convertValues(source["difficulties"], ProgressDifficultyResult);
	        this.explainedOnly = this.convertValues(source["explainedOnly"], ProgressConceptResult);
	        this.evolution = this.convertValues(source["evolution"], ProgressEvolutionResult);
	        this.insight = this.convertValues(source["insight"], ProgressInsightStateResult);
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
	
	export class QuizResultQuestionDTO {
	    id: string;
	    position: number;
	    kind: string;
	    prompt: string;
	    options: string[];
	    correctIndex: number;
	    explanation: string;
	    selectedIndex?: number;
	    correct: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QuizResultQuestionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.position = source["position"];
	        this.kind = source["kind"];
	        this.prompt = source["prompt"];
	        this.options = source["options"];
	        this.correctIndex = source["correctIndex"];
	        this.explanation = source["explanation"];
	        this.selectedIndex = source["selectedIndex"];
	        this.correct = source["correct"];
	    }
	}
	export class QuizAttemptSummaryDTO {
	    id: string;
	    quizId: string;
	    correctCount: number;
	    total: number;
	    score: number;
	    finishedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizAttemptSummaryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.quizId = source["quizId"];
	        this.correctCount = source["correctCount"];
	        this.total = source["total"];
	        this.score = source["score"];
	        this.finishedAt = source["finishedAt"];
	    }
	}
	export class QuizAttemptResultDTO {
	    attempt: QuizAttemptSummaryDTO;
	    sessionTitle: string;
	    difficulty: string;
	    questions: QuizResultQuestionDTO[];
	
	    static createFrom(source: any = {}) {
	        return new QuizAttemptResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attempt = this.convertValues(source["attempt"], QuizAttemptSummaryDTO);
	        this.sessionTitle = source["sessionTitle"];
	        this.difficulty = source["difficulty"];
	        this.questions = this.convertValues(source["questions"], QuizResultQuestionDTO);
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
	
	export class QuizQuestionDTO {
	    id: string;
	    position: number;
	    kind: string;
	    prompt: string;
	    options: string[];
	
	    static createFrom(source: any = {}) {
	        return new QuizQuestionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.position = source["position"];
	        this.kind = source["kind"];
	        this.prompt = source["prompt"];
	        this.options = source["options"];
	    }
	}
	export class QuizDTO {
	    id: string;
	    sessionTitle: string;
	    difficulty: string;
	    questions: QuizQuestionDTO[];
	
	    static createFrom(source: any = {}) {
	        return new QuizDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionTitle = source["sessionTitle"];
	        this.difficulty = source["difficulty"];
	        this.questions = this.convertValues(source["questions"], QuizQuestionDTO);
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
	export class QuizOfferDTO {
	    id: string;
	    sessionId: string;
	    messageId: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizOfferDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.messageId = source["messageId"];
	    }
	}
	
	
	export class QuizSelectionDTO {
	    questionId: string;
	    selectedIndex?: number;
	
	    static createFrom(source: any = {}) {
	        return new QuizSelectionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.questionId = source["questionId"];
	        this.selectedIndex = source["selectedIndex"];
	    }
	}
	export class QuizSummaryDTO {
	    id: string;
	    sessionId: string;
	    sessionTitle: string;
	    difficulty: string;
	    questionCount: number;
	    createdAt: string;
	    attemptCount: number;
	    latestScore?: number;
	    messageId: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizSummaryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.sessionTitle = source["sessionTitle"];
	        this.difficulty = source["difficulty"];
	        this.questionCount = source["questionCount"];
	        this.createdAt = source["createdAt"];
	        this.attemptCount = source["attemptCount"];
	        this.latestScore = source["latestScore"];
	        this.messageId = source["messageId"];
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
	export class SessionExerciseCardDTO {
	    sessionExerciseId: string;
	    messageId: string;
	    challengeId: string;
	    kind: string;
	    title: string;
	    difficulty: string;
	    timeLimitMinutes: number;
	    status: string;
	    attempts: number;
	    latest?: LatestResultDTO;
	
	    static createFrom(source: any = {}) {
	        return new SessionExerciseCardDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionExerciseId = source["sessionExerciseId"];
	        this.messageId = source["messageId"];
	        this.challengeId = source["challengeId"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.difficulty = source["difficulty"];
	        this.timeLimitMinutes = source["timeLimitMinutes"];
	        this.status = source["status"];
	        this.attempts = source["attempts"];
	        this.latest = this.convertValues(source["latest"], LatestResultDTO);
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
	export class SessionExerciseDTO {
	    challengeId: string;
	    kind: string;
	    title: string;
	    topic: string;
	    difficulty: string;
	    timeLimitMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionExerciseDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.challengeId = source["challengeId"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.topic = source["topic"];
	        this.difficulty = source["difficulty"];
	        this.timeLimitMinutes = source["timeLimitMinutes"];
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
	
	export class StudyProgressEvidenceResult {
	    assessment: string;
	    description: string;
	    assisted: boolean;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StudyProgressEvidenceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assessment = source["assessment"];
	        this.description = source["description"];
	        this.assisted = source["assisted"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class StudyProgressConceptResult {
	    name: string;
	    status: string;
	    assistanceCount: number;
	    evidenceCount: number;
	    updatedAt: string;
	    evidence: StudyProgressEvidenceResult[];
	
	    static createFrom(source: any = {}) {
	        return new StudyProgressConceptResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.assistanceCount = source["assistanceCount"];
	        this.evidenceCount = source["evidenceCount"];
	        this.updatedAt = source["updatedAt"];
	        this.evidence = this.convertValues(source["evidence"], StudyProgressEvidenceResult);
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
	
	export class StudyProgressResult {
	    demonstrated: StudyProgressConceptResult[];
	    explainedOnly: StudyProgressConceptResult[];
	    difficulties: StudyProgressConceptResult[];
	    needsReview: StudyProgressConceptResult[];
	
	    static createFrom(source: any = {}) {
	        return new StudyProgressResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.demonstrated = this.convertValues(source["demonstrated"], StudyProgressConceptResult);
	        this.explainedOnly = this.convertValues(source["explainedOnly"], StudyProgressConceptResult);
	        this.difficulties = this.convertValues(source["difficulties"], StudyProgressConceptResult);
	        this.needsReview = this.convertValues(source["needsReview"], StudyProgressConceptResult);
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

