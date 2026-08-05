export namespace main {
	
	export class VersionState {
	    version: string;
	    current: boolean;
	    initialized: boolean;
	    running: boolean;
	    pid: number;
	    port: number;
	
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
	        this.port = source["port"];
	    }
	}

}

export namespace postgres {
	
	export class ConfigFiles {
	    dataDir: string;
	    configFile: string;
	    hbaFile: string;
	    identFile: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigFiles(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataDir = source["dataDir"];
	        this.configFile = source["configFile"];
	        this.hbaFile = source["hbaFile"];
	        this.identFile = source["identFile"];
	    }
	}
	export class Extension {
	    name: string;
	    version: string;
	    installed: boolean;
	    comment: string;
	
	    static createFrom(source: any = {}) {
	        return new Extension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.installed = source["installed"];
	        this.comment = source["comment"];
	    }
	}
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

