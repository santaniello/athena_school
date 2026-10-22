export namespace desktop {
	
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

