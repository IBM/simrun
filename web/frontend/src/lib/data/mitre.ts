// MITRE ATT&CK Enterprise tactics and techniques reference data.
// Used for displaying human-readable names and building coverage matrices.

export interface MITRETactic {
	id: string;
	name: string;
	shortName: string;
}

export interface MITRETechnique {
	id: string;
	name: string;
	tactics: string[]; // tactic IDs this technique belongs to
}

export const MITRE_TACTICS: MITRETactic[] = [
	{ id: 'TA0043', name: 'Reconnaissance', shortName: 'Recon' },
	{ id: 'TA0042', name: 'Resource Development', shortName: 'Resource Dev' },
	{ id: 'TA0001', name: 'Initial Access', shortName: 'Initial Access' },
	{ id: 'TA0002', name: 'Execution', shortName: 'Execution' },
	{ id: 'TA0003', name: 'Persistence', shortName: 'Persistence' },
	{ id: 'TA0004', name: 'Privilege Escalation', shortName: 'Priv Esc' },
	{ id: 'TA0005', name: 'Defense Evasion', shortName: 'Def Evasion' },
	{ id: 'TA0006', name: 'Credential Access', shortName: 'Cred Access' },
	{ id: 'TA0007', name: 'Discovery', shortName: 'Discovery' },
	{ id: 'TA0008', name: 'Lateral Movement', shortName: 'Lateral Mov' },
	{ id: 'TA0009', name: 'Collection', shortName: 'Collection' },
	{ id: 'TA0011', name: 'Command and Control', shortName: 'C2' },
	{ id: 'TA0010', name: 'Exfiltration', shortName: 'Exfiltration' },
	{ id: 'TA0040', name: 'Impact', shortName: 'Impact' }
];

// Common cloud-relevant MITRE ATT&CK techniques.
// This is a curated subset focused on cloud attack techniques.
export const MITRE_TECHNIQUES: MITRETechnique[] = [
	// Reconnaissance
	{ id: 'T1595', name: 'Active Scanning', tactics: ['TA0043'] },
	{ id: 'T1592', name: 'Gather Victim Host Information', tactics: ['TA0043'] },
	{ id: 'T1589', name: 'Gather Victim Identity Information', tactics: ['TA0043'] },
	{ id: 'T1590', name: 'Gather Victim Network Information', tactics: ['TA0043'] },
	{ id: 'T1591', name: 'Gather Victim Org Information', tactics: ['TA0043'] },
	{ id: 'T1598', name: 'Phishing for Information', tactics: ['TA0043'] },
	{ id: 'T1597', name: 'Search Closed Sources', tactics: ['TA0043'] },
	{ id: 'T1596', name: 'Search Open Technical Databases', tactics: ['TA0043'] },
	{ id: 'T1593', name: 'Search Open Websites/Domains', tactics: ['TA0043'] },
	{ id: 'T1594', name: 'Search Victim-Owned Websites', tactics: ['TA0043'] },

	// Resource Development
	{ id: 'T1583', name: 'Acquire Infrastructure', tactics: ['TA0042'] },
	{ id: 'T1586', name: 'Compromise Accounts', tactics: ['TA0042'] },
	{ id: 'T1584', name: 'Compromise Infrastructure', tactics: ['TA0042'] },
	{ id: 'T1587', name: 'Develop Capabilities', tactics: ['TA0042'] },
	{ id: 'T1585', name: 'Establish Accounts', tactics: ['TA0042'] },
	{ id: 'T1588', name: 'Obtain Capabilities', tactics: ['TA0042'] },
	{ id: 'T1608', name: 'Stage Capabilities', tactics: ['TA0042'] },

	// Initial Access
	{ id: 'T1189', name: 'Drive-by Compromise', tactics: ['TA0001'] },
	{ id: 'T1190', name: 'Exploit Public-Facing Application', tactics: ['TA0001'] },
	{ id: 'T1133', name: 'External Remote Services', tactics: ['TA0001', 'TA0003'] },
	{ id: 'T1200', name: 'Hardware Additions', tactics: ['TA0001'] },
	{ id: 'T1566', name: 'Phishing', tactics: ['TA0001'] },
	{ id: 'T1091', name: 'Replication Through Removable Media', tactics: ['TA0001', 'TA0008'] },
	{ id: 'T1195', name: 'Supply Chain Compromise', tactics: ['TA0001'] },
	{ id: 'T1199', name: 'Trusted Relationship', tactics: ['TA0001'] },
	{ id: 'T1078', name: 'Valid Accounts', tactics: ['TA0001', 'TA0003', 'TA0004', 'TA0005'] },

	// Execution
	{ id: 'T1059', name: 'Command and Scripting Interpreter', tactics: ['TA0002'] },
	{ id: 'T1609', name: 'Container Administration Command', tactics: ['TA0002'] },
	{ id: 'T1610', name: 'Deploy Container', tactics: ['TA0002', 'TA0005'] },
	{ id: 'T1203', name: 'Exploitation for Client Execution', tactics: ['TA0002'] },
	{ id: 'T1559', name: 'Inter-Process Communication', tactics: ['TA0002'] },
	{ id: 'T1106', name: 'Native API', tactics: ['TA0002'] },
	{ id: 'T1053', name: 'Scheduled Task/Job', tactics: ['TA0002', 'TA0003', 'TA0004'] },
	{ id: 'T1648', name: 'Serverless Execution', tactics: ['TA0002'] },
	{ id: 'T1129', name: 'Shared Modules', tactics: ['TA0002'] },
	{ id: 'T1072', name: 'Software Deployment Tools', tactics: ['TA0002', 'TA0008'] },
	{ id: 'T1569', name: 'System Services', tactics: ['TA0002'] },
	{ id: 'T1204', name: 'User Execution', tactics: ['TA0002'] },

	// Persistence
	{ id: 'T1098', name: 'Account Manipulation', tactics: ['TA0003', 'TA0004'] },
	{ id: 'T1547', name: 'Boot or Logon Autostart Execution', tactics: ['TA0003', 'TA0004'] },
	{ id: 'T1037', name: 'Boot or Logon Initialization Scripts', tactics: ['TA0003', 'TA0004'] },
	{ id: 'T1136', name: 'Create Account', tactics: ['TA0003'] },
	{ id: 'T1543', name: 'Create or Modify System Process', tactics: ['TA0003', 'TA0004'] },
	{ id: 'T1546', name: 'Event Triggered Execution', tactics: ['TA0003', 'TA0004'] },
	{ id: 'T1574', name: 'Hijack Execution Flow', tactics: ['TA0003', 'TA0004', 'TA0005'] },
	{ id: 'T1525', name: 'Implant Internal Image', tactics: ['TA0003'] },
	{ id: 'T1556', name: 'Modify Authentication Process', tactics: ['TA0003', 'TA0005', 'TA0006'] },
	{ id: 'T1137', name: 'Office Application Startup', tactics: ['TA0003'] },
	{ id: 'T1542', name: 'Pre-OS Boot', tactics: ['TA0003', 'TA0005'] },
	{ id: 'T1505', name: 'Server Software Component', tactics: ['TA0003'] },
	{ id: 'T1205', name: 'Traffic Signaling', tactics: ['TA0003', 'TA0005'] },

	// Privilege Escalation
	{ id: 'T1548', name: 'Abuse Elevation Control Mechanism', tactics: ['TA0004', 'TA0005'] },
	{ id: 'T1134', name: 'Access Token Manipulation', tactics: ['TA0004', 'TA0005'] },
	{ id: 'T1611', name: 'Escape to Host', tactics: ['TA0004'] },
	{ id: 'T1068', name: 'Exploitation for Privilege Escalation', tactics: ['TA0004'] },
	{ id: 'T1055', name: 'Process Injection', tactics: ['TA0004', 'TA0005'] },

	// Defense Evasion
	{ id: 'T1550', name: 'Use Alternate Authentication Material', tactics: ['TA0005', 'TA0008'] },
	{ id: 'T1612', name: 'Build Image on Host', tactics: ['TA0005'] },
	{ id: 'T1140', name: 'Deobfuscate/Decode Files or Information', tactics: ['TA0005'] },
	{ id: 'T1006', name: 'Direct Volume Access', tactics: ['TA0005'] },
	{ id: 'T1484', name: 'Domain Policy Modification', tactics: ['TA0005', 'TA0004'] },
	{ id: 'T1480', name: 'Execution Guardrails', tactics: ['TA0005'] },
	{ id: 'T1211', name: 'Exploitation for Defense Evasion', tactics: ['TA0005'] },
	{ id: 'T1222', name: 'File and Directory Permissions Modification', tactics: ['TA0005'] },
	{ id: 'T1564', name: 'Hide Artifacts', tactics: ['TA0005'] },
	{ id: 'T1562', name: 'Impair Defenses', tactics: ['TA0005'] },
	{ id: 'T1070', name: 'Indicator Removal', tactics: ['TA0005'] },
	{ id: 'T1202', name: 'Indirect Command Execution', tactics: ['TA0005'] },
	{ id: 'T1036', name: 'Masquerading', tactics: ['TA0005'] },
	{ id: 'T1556', name: 'Modify Authentication Process', tactics: ['TA0003', 'TA0005', 'TA0006'] },
	{ id: 'T1578', name: 'Modify Cloud Compute Infrastructure', tactics: ['TA0005'] },
	{ id: 'T1112', name: 'Modify Registry', tactics: ['TA0005'] },
	{ id: 'T1601', name: 'Modify System Image', tactics: ['TA0005'] },
	{ id: 'T1599', name: 'Network Boundary Bridging', tactics: ['TA0005'] },
	{ id: 'T1027', name: 'Obfuscated Files or Information', tactics: ['TA0005'] },
	{ id: 'T1647', name: 'Plist File Modification', tactics: ['TA0005'] },
	{ id: 'T1207', name: 'Rogue Domain Controller', tactics: ['TA0005'] },
	{ id: 'T1014', name: 'Rootkit', tactics: ['TA0005'] },
	{ id: 'T1218', name: 'System Binary Proxy Execution', tactics: ['TA0005'] },
	{ id: 'T1216', name: 'System Script Proxy Execution', tactics: ['TA0005'] },
	{ id: 'T1221', name: 'Template Injection', tactics: ['TA0005'] },
	{ id: 'T1127', name: 'Trusted Developer Utilities Proxy Execution', tactics: ['TA0005'] },
	{ id: 'T1535', name: 'Unused/Unsupported Cloud Regions', tactics: ['TA0005'] },
	{ id: 'T1497', name: 'Virtualization/Sandbox Evasion', tactics: ['TA0005', 'TA0007'] },
	{ id: 'T1600', name: 'Weaken Encryption', tactics: ['TA0005'] },
	{ id: 'T1220', name: 'XSL Script Processing', tactics: ['TA0005'] },

	// Credential Access
	{ id: 'T1557', name: 'Adversary-in-the-Middle', tactics: ['TA0006', 'TA0009'] },
	{ id: 'T1110', name: 'Brute Force', tactics: ['TA0006'] },
	{ id: 'T1555', name: 'Credentials from Password Stores', tactics: ['TA0006'] },
	{ id: 'T1212', name: 'Exploitation for Credential Access', tactics: ['TA0006'] },
	{ id: 'T1187', name: 'Forced Authentication', tactics: ['TA0006'] },
	{ id: 'T1606', name: 'Forge Web Credentials', tactics: ['TA0006'] },
	{ id: 'T1056', name: 'Input Capture', tactics: ['TA0006', 'TA0009'] },
	{ id: 'T1649', name: 'Steal or Forge Authentication Certificates', tactics: ['TA0006'] },
	{ id: 'T1552', name: 'Unsecured Credentials', tactics: ['TA0006'] },
	{ id: 'T1558', name: 'Steal or Forge Kerberos Tickets', tactics: ['TA0006'] },
	{ id: 'T1539', name: 'Steal Web Session Cookie', tactics: ['TA0006'] },
	{ id: 'T1528', name: 'Steal Application Access Token', tactics: ['TA0006'] },
	{ id: 'T1003', name: 'OS Credential Dumping', tactics: ['TA0006'] },
	{ id: 'T1621', name: 'Multi-Factor Authentication Request Generation', tactics: ['TA0006'] },

	// Discovery
	{ id: 'T1087', name: 'Account Discovery', tactics: ['TA0007'] },
	{ id: 'T1010', name: 'Application Window Discovery', tactics: ['TA0007'] },
	{ id: 'T1217', name: 'Browser Information Discovery', tactics: ['TA0007'] },
	{ id: 'T1580', name: 'Cloud Infrastructure Discovery', tactics: ['TA0007'] },
	{ id: 'T1538', name: 'Cloud Service Dashboard', tactics: ['TA0007'] },
	{ id: 'T1526', name: 'Cloud Service Discovery', tactics: ['TA0007'] },
	{ id: 'T1613', name: 'Container and Resource Discovery', tactics: ['TA0007'] },
	{ id: 'T1482', name: 'Domain Trust Discovery', tactics: ['TA0007'] },
	{ id: 'T1083', name: 'File and Directory Discovery', tactics: ['TA0007'] },
	{ id: 'T1615', name: 'Group Policy Discovery', tactics: ['TA0007'] },
	{ id: 'T1046', name: 'Network Service Discovery', tactics: ['TA0007'] },
	{ id: 'T1135', name: 'Network Share Discovery', tactics: ['TA0007'] },
	{ id: 'T1040', name: 'Network Sniffing', tactics: ['TA0007', 'TA0006'] },
	{ id: 'T1201', name: 'Password Policy Discovery', tactics: ['TA0007'] },
	{ id: 'T1120', name: 'Peripheral Device Discovery', tactics: ['TA0007'] },
	{ id: 'T1069', name: 'Permission Groups Discovery', tactics: ['TA0007'] },
	{ id: 'T1057', name: 'Process Discovery', tactics: ['TA0007'] },
	{ id: 'T1012', name: 'Query Registry', tactics: ['TA0007'] },
	{ id: 'T1018', name: 'Remote System Discovery', tactics: ['TA0007'] },
	{ id: 'T1518', name: 'Software Discovery', tactics: ['TA0007'] },
	{ id: 'T1082', name: 'System Information Discovery', tactics: ['TA0007'] },
	{ id: 'T1614', name: 'System Location Discovery', tactics: ['TA0007'] },
	{ id: 'T1016', name: 'System Network Configuration Discovery', tactics: ['TA0007'] },
	{ id: 'T1049', name: 'System Network Connections Discovery', tactics: ['TA0007'] },
	{ id: 'T1033', name: 'System Owner/User Discovery', tactics: ['TA0007'] },
	{ id: 'T1007', name: 'System Service Discovery', tactics: ['TA0007'] },
	{ id: 'T1124', name: 'System Time Discovery', tactics: ['TA0007'] },

	// Lateral Movement
	{ id: 'T1210', name: 'Exploitation of Remote Services', tactics: ['TA0008'] },
	{ id: 'T1534', name: 'Internal Spearphishing', tactics: ['TA0008'] },
	{ id: 'T1570', name: 'Lateral Tool Transfer', tactics: ['TA0008'] },
	{ id: 'T1563', name: 'Remote Service Session Hijacking', tactics: ['TA0008'] },
	{ id: 'T1021', name: 'Remote Services', tactics: ['TA0008'] },
	{ id: 'T1080', name: 'Taint Shared Content', tactics: ['TA0008'] },

	// Collection
	{ id: 'T1560', name: 'Archive Collected Data', tactics: ['TA0009'] },
	{ id: 'T1123', name: 'Audio Capture', tactics: ['TA0009'] },
	{ id: 'T1119', name: 'Automated Collection', tactics: ['TA0009'] },
	{ id: 'T1185', name: 'Browser Session Hijacking', tactics: ['TA0009'] },
	{ id: 'T1115', name: 'Clipboard Data', tactics: ['TA0009'] },
	{ id: 'T1530', name: 'Data from Cloud Storage', tactics: ['TA0009'] },
	{ id: 'T1602', name: 'Data from Configuration Repository', tactics: ['TA0009'] },
	{ id: 'T1213', name: 'Data from Information Repositories', tactics: ['TA0009'] },
	{ id: 'T1005', name: 'Data from Local System', tactics: ['TA0009'] },
	{ id: 'T1039', name: 'Data from Network Shared Drive', tactics: ['TA0009'] },
	{ id: 'T1025', name: 'Data from Removable Media', tactics: ['TA0009'] },
	{ id: 'T1074', name: 'Data Staged', tactics: ['TA0009'] },
	{ id: 'T1114', name: 'Email Collection', tactics: ['TA0009'] },
	{ id: 'T1113', name: 'Screen Capture', tactics: ['TA0009'] },
	{ id: 'T1125', name: 'Video Capture', tactics: ['TA0009'] },

	// Command and Control
	{ id: 'T1071', name: 'Application Layer Protocol', tactics: ['TA0011'] },
	{ id: 'T1092', name: 'Communication Through Removable Media', tactics: ['TA0011'] },
	{ id: 'T1132', name: 'Data Encoding', tactics: ['TA0011'] },
	{ id: 'T1001', name: 'Data Obfuscation', tactics: ['TA0011'] },
	{ id: 'T1568', name: 'Dynamic Resolution', tactics: ['TA0011'] },
	{ id: 'T1573', name: 'Encrypted Channel', tactics: ['TA0011'] },
	{ id: 'T1008', name: 'Fallback Channels', tactics: ['TA0011'] },
	{ id: 'T1105', name: 'Ingress Tool Transfer', tactics: ['TA0011'] },
	{ id: 'T1104', name: 'Multi-Stage Channels', tactics: ['TA0011'] },
	{ id: 'T1095', name: 'Non-Application Layer Protocol', tactics: ['TA0011'] },
	{ id: 'T1571', name: 'Non-Standard Port', tactics: ['TA0011'] },
	{ id: 'T1572', name: 'Protocol Tunneling', tactics: ['TA0011'] },
	{ id: 'T1090', name: 'Proxy', tactics: ['TA0011'] },
	{ id: 'T1219', name: 'Remote Access Software', tactics: ['TA0011'] },
	{ id: 'T1102', name: 'Web Service', tactics: ['TA0011'] },

	// Exfiltration
	{ id: 'T1020', name: 'Automated Exfiltration', tactics: ['TA0010'] },
	{ id: 'T1030', name: 'Data Transfer Size Limits', tactics: ['TA0010'] },
	{ id: 'T1048', name: 'Exfiltration Over Alternative Protocol', tactics: ['TA0010'] },
	{ id: 'T1041', name: 'Exfiltration Over C2 Channel', tactics: ['TA0010'] },
	{ id: 'T1011', name: 'Exfiltration Over Other Network Medium', tactics: ['TA0010'] },
	{ id: 'T1052', name: 'Exfiltration Over Physical Medium', tactics: ['TA0010'] },
	{ id: 'T1567', name: 'Exfiltration Over Web Service', tactics: ['TA0010'] },
	{ id: 'T1029', name: 'Scheduled Transfer', tactics: ['TA0010'] },
	{ id: 'T1537', name: 'Transfer Data to Cloud Account', tactics: ['TA0010'] },

	// Impact
	{ id: 'T1531', name: 'Account Access Removal', tactics: ['TA0040'] },
	{ id: 'T1485', name: 'Data Destruction', tactics: ['TA0040'] },
	{ id: 'T1486', name: 'Data Encrypted for Impact', tactics: ['TA0040'] },
	{ id: 'T1565', name: 'Data Manipulation', tactics: ['TA0040'] },
	{ id: 'T1491', name: 'Defacement', tactics: ['TA0040'] },
	{ id: 'T1561', name: 'Disk Wipe', tactics: ['TA0040'] },
	{ id: 'T1499', name: 'Endpoint Denial of Service', tactics: ['TA0040'] },
	{ id: 'T1657', name: 'Financial Theft', tactics: ['TA0040'] },
	{ id: 'T1495', name: 'Firmware Corruption', tactics: ['TA0040'] },
	{ id: 'T1490', name: 'Inhibit System Recovery', tactics: ['TA0040'] },
	{ id: 'T1498', name: 'Network Denial of Service', tactics: ['TA0040'] },
	{ id: 'T1496', name: 'Resource Hijacking', tactics: ['TA0040'] },
	{ id: 'T1489', name: 'Service Stop', tactics: ['TA0040'] },
	{ id: 'T1529', name: 'System Shutdown/Reboot', tactics: ['TA0040'] }
];

// Lookup maps for quick access
const tacticMap = new Map(MITRE_TACTICS.map((t) => [t.id, t]));
const techniqueMap = new Map(MITRE_TECHNIQUES.map((t) => [t.id, t]));

export function getTacticName(id: string): string {
	return tacticMap.get(id)?.name ?? id;
}

export function getTacticShortName(id: string): string {
	return tacticMap.get(id)?.shortName ?? id;
}

export function getTechniqueName(id: string): string {
	return techniqueMap.get(id)?.name ?? id;
}

export function getTechnique(id: string): MITRETechnique | undefined {
	return techniqueMap.get(id);
}

export function getTactic(id: string): MITRETactic | undefined {
	return tacticMap.get(id);
}
