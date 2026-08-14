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

}

