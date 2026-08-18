export namespace desktop {
	
	export class FolderResult {
	    id: string;
	    name: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FolderResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.isDefault = source["isDefault"];
	    }
	}
	export class LoginResult {
	    accountId: string;
	    email: string;
	
	    static createFrom(source: any = {}) {
	        return new LoginResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.email = source["email"];
	    }
	}
	export class StudyMessageResult {
	    role: string;
	    content: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StudyMessageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class StudySessionResult {
	    id: string;
	    topic: string;
	    folderId: string;
	    startedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StudySessionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.topic = source["topic"];
	        this.folderId = source["folderId"];
	        this.startedAt = source["startedAt"];
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
	    goals: string[];
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
	        this.goals = source["goals"];
	        this.studyStyle = source["studyStyle"];
	        this.assistantLanguage = source["assistantLanguage"];
	    }
	}

}

