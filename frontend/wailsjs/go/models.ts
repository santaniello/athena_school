export namespace desktop {
	
	export class ChunkLoadIssueResult {
	    chunkId: string;
	    itemId: string;
	    source: string;
	    filePath: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ChunkLoadIssueResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chunkId = source["chunkId"];
	        this.itemId = source["itemId"];
	        this.source = source["source"];
	        this.filePath = source["filePath"];
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
	export class KnowledgeEvidenceResult {
	    originType: string;
	    sourceLabel: string;
	    excerpt: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeEvidenceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.originType = source["originType"];
	        this.sourceLabel = source["sourceLabel"];
	        this.excerpt = source["excerpt"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class KnowledgeItemInput {
	    id: string;
	    topic: string;
	    concept: string;
	    definition: string;
	    properties: string[];
	    tradeOffs: string[];
	    relatedConcepts: string[];
	    source: string;
	    status: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeItemInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.topic = source["topic"];
	        this.concept = source["concept"];
	        this.definition = source["definition"];
	        this.properties = source["properties"];
	        this.tradeOffs = source["tradeOffs"];
	        this.relatedConcepts = source["relatedConcepts"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class KnowledgeItemResult {
	    id: string;
	    topic: string;
	    concept: string;
	    definition: string;
	    properties: string[];
	    tradeOffs: string[];
	    relatedConcepts: string[];
	    source: string;
	    status: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.topic = source["topic"];
	        this.concept = source["concept"];
	        this.definition = source["definition"];
	        this.properties = source["properties"];
	        this.tradeOffs = source["tradeOffs"];
	        this.relatedConcepts = source["relatedConcepts"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
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
	    sourceType: string;
	    filePath: string;
	    heading: string;
	    concept: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new StudySourceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceType = source["sourceType"];
	        this.filePath = source["filePath"];
	        this.heading = source["heading"];
	        this.concept = source["concept"];
	        this.score = source["score"];
	    }
	}
	export class StudyMessageResult {
	    role: string;
	    content: string;
	    createdAt: string;
	    sources: StudySourceResult[];
	
	    static createFrom(source: any = {}) {
	        return new StudyMessageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
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
	
	    static createFrom(source: any = {}) {
	        return new StudySessionHistoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session = this.convertValues(source["session"], StudySessionResult);
	        this.messages = this.convertValues(source["messages"], StudyMessageResult);
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

