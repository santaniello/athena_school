export namespace desktop {
	
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
	export class StudySessionResult {
	    id: string;
	    topic: string;
	    startedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StudySessionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.topic = source["topic"];
	        this.startedAt = source["startedAt"];
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

