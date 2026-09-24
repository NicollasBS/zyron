export namespace config {
	
	export class Config {
	    server_ip: string;
	    server_port: number;
	    server_url: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_ip = source["server_ip"];
	        this.server_port = source["server_port"];
	        this.server_url = source["server_url"];
	    }
	}

}

export namespace main {
	
	export class CheckProfileResponse {
	    is_operator: boolean;
	    username: string;
	    display_name: string;
	    success: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckProfileResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_operator = source["is_operator"];
	        this.username = source["username"];
	        this.display_name = source["display_name"];
	        this.success = source["success"];
	        this.error = source["error"];
	    }
	}

}

export namespace system {
	
	export class SystemInfo {
	    username: string;
	    hostname: string;
	    ip: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.hostname = source["hostname"];
	        this.ip = source["ip"];
	    }
	}

}

export namespace updater {
	
	export class UpdateApplyResult {
	    success: boolean;
	    message: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.error = source["error"];
	    }
	}
	export class UpdateCheckResult {
	    update_available: boolean;
	    current_version: string;
	    latest_version: string;
	    download_url: string;
	    release_notes: string;
	    size: number;
	    sha256: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.update_available = source["update_available"];
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.download_url = source["download_url"];
	        this.release_notes = source["release_notes"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	        this.error = source["error"];
	    }
	}

}

