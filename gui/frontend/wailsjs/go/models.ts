export namespace main {
	
	export class VersionState {
	    version: string;
	    current: boolean;
	    initialized: boolean;
	    running: boolean;
	    pid: number;
	
	    static createFrom(source: any = {}) {
	        return new VersionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.current = source["current"];
	        this.initialized = source["initialized"];
	        this.running = source["running"];
	        this.pid = source["pid"];
	    }
	}

}

export namespace postgres {
	
	export class Version {
	    version: string;
	    current_minor: string;
	    supported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Version(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.current_minor = source["current_minor"];
	        this.supported = source["supported"];
	    }
	}

}

