CREATE DATABASE IF NOT EXISTS churn;
DROP TABLE IF EXISTS churn.train;
CREATE TABLE  churn.train (
	customerID VARCHAR(255),
	gender VARCHAR(255),
	SeniorCitizen TINYINT,
	Partner VARCHAR(255),
	Dependents VARCHAR(255),
	tenure INT,
	PhoneService VARCHAR(255),
	MultipleLines VARCHAR(255),
	InternetService VARCHAR(255),
	OnlineSecurity VARCHAR(255),
	OnlineBackup VARCHAR(255),
	DeviceProtection VARCHAR(255),
	TechSupport VARCHAR(255),
	StreamingTV VARCHAR(255),
	StreamingMovies VARCHAR(255),
	Contract VARCHAR(255),
	PaperlessBilling VARCHAR(255),
	PaymentMethod VARCHAR(255),
	MonthlyCharges FLOAT,
	TotalCharges FLOAT,
	Churn VARCHAR(255),
	PRIMARY KEY ( customerID )
);
INSERT INTO churn.train VALUES("7590-VHVEG","Female","0","Yes","No","1","No","No phone service","DSL","No","Yes","No","No","No","No","Month-to-month","Yes","Electronic check","29.85","29.85","No");
INSERT INTO churn.train VALUES("5575-GNVDE","Male","0","No","No","34","Yes","No","DSL","Yes","No","Yes","No","No","No","One year","No","Mailed check","56.95","1889.5","No");
INSERT INTO churn.train VALUES("3668-QPYBK","Male","0","No","No","2","Yes","No","DSL","Yes","Yes","No","No","No","No","Month-to-month","Yes","Mailed check","53.85","108.15","Yes");
INSERT INTO churn.train VALUES("7795-CFOCW","Male","0","No","No","45","No","No phone service","DSL","Yes","No","Yes","Yes","No","No","One year","No","Bank transfer (automatic)","42.3","1840.75","No");
INSERT INTO churn.train VALUES("9237-HQITU","Female","0","No","No","2","Yes","No","Fiber optic","No","No","No","No","No","No","Month-to-month","Yes","Electronic check","70.7","151.65","Yes");
INSERT INTO churn.train VALUES("9305-CDSKC","Female","0","No","No","8","Yes","Yes","Fiber optic","No","No","Yes","No","Yes","Yes","Month-to-month","Yes","Electronic check","99.65","820.5","Yes");
INSERT INTO churn.train VALUES("1452-KIOVK","Male","0","No","Yes","22","Yes","Yes","Fiber optic","No","Yes","No","No","Yes","No","Month-to-month","Yes","Credit card (automatic)","89.1","1949.4","No");
INSERT INTO churn.train VALUES("6713-OKOMC","Female","0","No","No","10","No","No phone service","DSL","Yes","No","No","No","No","No","Month-to-month","No","Mailed check","29.75","301.9","No");
INSERT INTO churn.train VALUES("7892-POOKP","Female","0","Yes","No","28","Yes","Yes","Fiber optic","No","No","Yes","Yes","Yes","Yes","Month-to-month","Yes","Electronic check","104.8","3046.05","Yes");
INSERT INTO churn.train VALUES("6388-TABGU","Male","0","No","Yes","62","Yes","No","DSL","Yes","Yes","No","No","No","No","One year","No","Bank transfer (automatic)","56.15","3487.95","No");
INSERT INTO churn.train VALUES("9763-GRSKD","Male","0","Yes","Yes","13","Yes","No","DSL","Yes","No","No","No","No","No","Month-to-month","Yes","Mailed check","49.95","587.45","No");
INSERT INTO churn.train VALUES("7469-LKBCI","Male","0","No","No","16","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Two year","No","Credit card (automatic)","18.95","326.8","No");
INSERT INTO churn.train VALUES("8091-TTVAX","Male","0","Yes","No","58","Yes","Yes","Fiber optic","No","No","Yes","No","Yes","Yes","One year","No","Credit card (automatic)","100.35","5681.1","No");
INSERT INTO churn.train VALUES("0280-XJGEX","Male","0","No","No","49","Yes","Yes","Fiber optic","No","Yes","Yes","No","Yes","Yes","Month-to-month","Yes","Bank transfer (automatic)","103.7","5036.3","Yes");
INSERT INTO churn.train VALUES("5129-JLPIS","Male","0","No","No","25","Yes","No","Fiber optic","Yes","No","Yes","Yes","Yes","Yes","Month-to-month","Yes","Electronic check","105.5","2686.05","No");
INSERT INTO churn.train VALUES("3655-SNQYZ","Female","0","Yes","Yes","69","Yes","Yes","Fiber optic","Yes","Yes","Yes","Yes","Yes","Yes","Two year","No","Credit card (automatic)","113.25","7895.15","No");
INSERT INTO churn.train VALUES("8191-XWSZG","Female","0","No","No","52","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","One year","No","Mailed check","20.65","1022.95","No");
INSERT INTO churn.train VALUES("9959-WOFKT","Male","0","No","Yes","71","Yes","Yes","Fiber optic","Yes","No","Yes","No","Yes","Yes","Two year","No","Bank transfer (automatic)","106.7","7382.25","No");
INSERT INTO churn.train VALUES("4190-MFLUW","Female","0","Yes","Yes","10","Yes","No","DSL","No","No","Yes","Yes","No","No","Month-to-month","No","Credit card (automatic)","55.2","528.35","Yes");
INSERT INTO churn.train VALUES("4183-MYFRB","Female","0","No","No","21","Yes","No","Fiber optic","No","Yes","Yes","No","No","Yes","Month-to-month","Yes","Electronic check","90.05","1862.9","No");
INSERT INTO churn.train VALUES("8779-QRDMV","Male","1","No","No","1","No","No phone service","DSL","No","No","Yes","No","No","Yes","Month-to-month","Yes","Electronic check","39.65","39.65","Yes");
INSERT INTO churn.train VALUES("1680-VDCWW","Male","0","Yes","No","12","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","One year","No","Bank transfer (automatic)","19.8","202.25","No");
INSERT INTO churn.train VALUES("1066-JKSGK","Male","0","No","No","1","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Month-to-month","No","Mailed check","20.15","20.15","Yes");
INSERT INTO churn.train VALUES("3638-WEABW","Female","0","Yes","No","58","Yes","Yes","DSL","No","Yes","No","Yes","No","No","Two year","Yes","Credit card (automatic)","59.9","3505.1","No");
INSERT INTO churn.train VALUES("6322-HRPFA","Male","0","Yes","Yes","49","Yes","No","DSL","Yes","Yes","No","Yes","No","No","Month-to-month","No","Credit card (automatic)","59.6","2970.3","No");
INSERT INTO churn.train VALUES("6865-JZNKO","Female","0","No","No","30","Yes","No","DSL","Yes","Yes","No","No","No","No","Month-to-month","Yes","Bank transfer (automatic)","55.3","1530.6","No");
INSERT INTO churn.train VALUES("6467-CHFZW","Male","0","Yes","Yes","47","Yes","Yes","Fiber optic","No","Yes","No","No","Yes","Yes","Month-to-month","Yes","Electronic check","99.35","4749.15","Yes");
INSERT INTO churn.train VALUES("8665-UTDHZ","Male","0","Yes","Yes","1","No","No phone service","DSL","No","Yes","No","No","No","No","Month-to-month","No","Electronic check","30.2","30.2","Yes");
INSERT INTO churn.train VALUES("5248-YGIJN","Male","0","Yes","No","72","Yes","Yes","DSL","Yes","Yes","Yes","Yes","Yes","Yes","Two year","Yes","Credit card (automatic)","90.25","6369.45","No");
INSERT INTO churn.train VALUES("8773-HHUOZ","Female","0","No","Yes","17","Yes","No","DSL","No","No","No","No","Yes","Yes","Month-to-month","Yes","Mailed check","64.7","1093.1","Yes");
INSERT INTO churn.train VALUES("3841-NFECX","Female","1","Yes","No","71","Yes","Yes","Fiber optic","Yes","Yes","Yes","Yes","No","No","Two year","Yes","Credit card (automatic)","96.35","6766.95","No");
INSERT INTO churn.train VALUES("4929-XIHVW","Male","1","Yes","No","2","Yes","No","Fiber optic","No","No","Yes","No","Yes","Yes","Month-to-month","Yes","Credit card (automatic)","95.5","181.65","No");
INSERT INTO churn.train VALUES("6827-IEAUQ","Female","0","Yes","Yes","27","Yes","No","DSL","Yes","Yes","Yes","Yes","No","No","One year","No","Mailed check","66.15","1874.45","No");
INSERT INTO churn.train VALUES("7310-EGVHZ","Male","0","No","No","1","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Month-to-month","No","Bank transfer (automatic)","20.2","20.2","No");
INSERT INTO churn.train VALUES("3413-BMNZE","Male","1","No","No","1","Yes","No","DSL","No","No","No","No","No","No","Month-to-month","No","Bank transfer (automatic)","45.25","45.25","No");
INSERT INTO churn.train VALUES("6234-RAAPL","Female","0","Yes","Yes","72","Yes","Yes","Fiber optic","Yes","Yes","No","Yes","Yes","No","Two year","No","Bank transfer (automatic)","99.9","7251.7","No");
INSERT INTO churn.train VALUES("6047-YHPVI","Male","0","No","No","5","Yes","No","Fiber optic","No","No","No","No","No","No","Month-to-month","Yes","Electronic check","69.7","316.9","Yes");
INSERT INTO churn.train VALUES("6572-ADKRS","Female","0","No","No","46","Yes","No","Fiber optic","No","No","Yes","No","No","No","Month-to-month","Yes","Credit card (automatic)","74.8","3548.3","No");
INSERT INTO churn.train VALUES("5380-WJKOV","Male","0","No","No","34","Yes","Yes","Fiber optic","No","Yes","Yes","No","Yes","Yes","Month-to-month","Yes","Electronic check","106.35","3549.25","Yes");
INSERT INTO churn.train VALUES("8168-UQWWF","Female","0","No","No","11","Yes","Yes","Fiber optic","No","No","Yes","No","Yes","Yes","Month-to-month","Yes","Bank transfer (automatic)","97.85","1105.4","Yes");
INSERT INTO churn.train VALUES("8865-TNMNX","Male","0","Yes","Yes","10","Yes","No","DSL","No","Yes","No","No","No","No","One year","No","Mailed check","49.55","475.7","No");
INSERT INTO churn.train VALUES("9489-DEDVP","Female","0","Yes","Yes","70","Yes","Yes","DSL","Yes","Yes","No","No","Yes","No","Two year","Yes","Credit card (automatic)","69.2","4872.35","No");
INSERT INTO churn.train VALUES("9867-JCZSP","Female","0","Yes","Yes","17","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","One year","No","Mailed check","20.75","418.25","No");
INSERT INTO churn.train VALUES("4671-VJLCL","Female","0","No","No","63","Yes","Yes","DSL","Yes","Yes","Yes","Yes","Yes","No","Two year","Yes","Credit card (automatic)","79.85","4861.45","No");
INSERT INTO churn.train VALUES("4080-IIARD","Female","0","Yes","No","13","Yes","Yes","DSL","Yes","Yes","No","Yes","Yes","No","Month-to-month","Yes","Electronic check","76.2","981.45","No");
INSERT INTO churn.train VALUES("3714-NTNFO","Female","0","No","No","49","Yes","Yes","Fiber optic","No","No","No","No","No","Yes","Month-to-month","Yes","Electronic check","84.5","3906.7","No");
INSERT INTO churn.train VALUES("5948-UJZLF","Male","0","No","No","2","Yes","No","DSL","No","Yes","No","No","No","No","Month-to-month","No","Mailed check","49.25","97","No");
INSERT INTO churn.train VALUES("7760-OYPDY","Female","0","No","No","2","Yes","No","Fiber optic","No","No","No","No","Yes","No","Month-to-month","Yes","Electronic check","80.65","144.15","Yes");
INSERT INTO churn.train VALUES("7639-LIAYI","Male","0","No","No","52","Yes","Yes","DSL","Yes","No","No","Yes","Yes","Yes","Two year","Yes","Credit card (automatic)","79.75","4217.8","No");
INSERT INTO churn.train VALUES("2954-PIBKO","Female","0","Yes","Yes","69","Yes","Yes","DSL","Yes","No","Yes","Yes","No","No","Two year","Yes","Credit card (automatic)","64.15","4254.1","No");
INSERT INTO churn.train VALUES("8012-SOUDQ","Female","1","No","No","43","Yes","Yes","Fiber optic","No","Yes","No","No","Yes","No","Month-to-month","Yes","Electronic check","90.25","3838.75","No");
INSERT INTO churn.train VALUES("9420-LOJKX","Female","0","No","No","15","Yes","No","Fiber optic","Yes","Yes","No","No","Yes","Yes","Month-to-month","Yes","Credit card (automatic)","99.1","1426.4","Yes");
INSERT INTO churn.train VALUES("6575-SUVOI","Female","1","Yes","No","25","Yes","Yes","DSL","Yes","No","No","Yes","Yes","No","Month-to-month","Yes","Credit card (automatic)","69.5","1752.65","No");
INSERT INTO churn.train VALUES("7495-OOKFY","Female","1","Yes","No","8","Yes","Yes","Fiber optic","No","Yes","No","No","No","No","Month-to-month","Yes","Credit card (automatic)","80.65","633.3","Yes");
INSERT INTO churn.train VALUES("4667-QONEA","Female","1","Yes","Yes","60","Yes","No","DSL","Yes","Yes","Yes","Yes","No","Yes","One year","Yes","Credit card (automatic)","74.85","4456.35","No");
INSERT INTO churn.train VALUES("1658-BYGOY","Male","1","No","No","18","Yes","Yes","Fiber optic","No","No","No","No","Yes","Yes","Month-to-month","Yes","Electronic check","95.45","1752.55","Yes");
INSERT INTO churn.train VALUES("8769-KKTPH","Female","0","Yes","Yes","63","Yes","Yes","Fiber optic","Yes","No","No","No","Yes","Yes","One year","Yes","Credit card (automatic)","99.65","6311.2","No");
INSERT INTO churn.train VALUES("5067-XJQFU","Male","1","Yes","Yes","66","Yes","Yes","Fiber optic","No","Yes","Yes","Yes","Yes","Yes","One year","Yes","Electronic check","108.45","7076.35","No");
INSERT INTO churn.train VALUES("3957-SQXML","Female","0","Yes","Yes","34","Yes","Yes","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Two year","No","Credit card (automatic)","24.95","894.3","No");
INSERT INTO churn.train VALUES("5954-BDFSG","Female","0","No","No","72","Yes","Yes","Fiber optic","No","No","Yes","Yes","Yes","Yes","Two year","Yes","Credit card (automatic)","107.5","7853.7","No");
INSERT INTO churn.train VALUES("0434-CSFON","Female","0","Yes","No","47","Yes","Yes","Fiber optic","No","No","Yes","No","Yes","Yes","Month-to-month","Yes","Electronic check","100.5","4707.1","No");
INSERT INTO churn.train VALUES("1215-FIGMP","Male","0","No","No","60","Yes","Yes","Fiber optic","No","Yes","No","No","Yes","No","Month-to-month","Yes","Bank transfer (automatic)","89.9","5450.7","No");
INSERT INTO churn.train VALUES("0526-SXDJP","Male","0","Yes","No","72","No","No phone service","DSL","Yes","Yes","Yes","No","No","No","Two year","No","Bank transfer (automatic)","42.1","2962","No");
INSERT INTO churn.train VALUES("0557-ASKVU","Female","0","Yes","Yes","18","Yes","No","DSL","No","No","Yes","Yes","No","No","One year","Yes","Credit card (automatic)","54.4","957.1","No");
INSERT INTO churn.train VALUES("5698-BQJOH","Female","0","No","No","9","Yes","Yes","Fiber optic","No","No","No","No","Yes","Yes","Month-to-month","No","Electronic check","94.4","857.25","Yes");
INSERT INTO churn.train VALUES("5122-CYFXA","Female","0","No","No","3","Yes","No","DSL","No","Yes","No","Yes","Yes","Yes","Month-to-month","Yes","Electronic check","75.3","244.1","No");
INSERT INTO churn.train VALUES("8627-ZYGSZ","Male","0","Yes","No","47","Yes","Yes","Fiber optic","No","Yes","No","No","No","No","One year","Yes","Electronic check","78.9","3650.35","No");
INSERT INTO churn.train VALUES("3410-YOQBQ","Female","0","No","No","31","Yes","No","DSL","No","Yes","Yes","Yes","Yes","Yes","Two year","No","Mailed check","79.2","2497.2","No");
INSERT INTO churn.train VALUES("3170-NMYVV","Female","0","Yes","Yes","50","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Two year","No","Bank transfer (automatic)","20.15","930.9","No");
INSERT INTO churn.train VALUES("7410-OIEDU","Male","0","No","No","10","Yes","No","Fiber optic","Yes","No","Yes","No","No","No","Month-to-month","Yes","Mailed check","79.85","887.35","No");
INSERT INTO churn.train VALUES("2273-QCKXA","Male","0","No","No","1","Yes","No","DSL","No","No","No","Yes","No","No","Month-to-month","No","Mailed check","49.05","49.05","No");
INSERT INTO churn.train VALUES("0731-EBJQB","Female","0","Yes","Yes","52","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","One year","Yes","Electronic check","20.4","1090.65","No");
INSERT INTO churn.train VALUES("1891-QRQSA","Male","1","Yes","Yes","64","Yes","Yes","Fiber optic","Yes","No","Yes","Yes","Yes","Yes","Two year","Yes","Bank transfer (automatic)","111.6","7099","No");
INSERT INTO churn.train VALUES("8028-PNXHQ","Male","0","Yes","Yes","62","Yes","Yes","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Two year","Yes","Bank transfer (automatic)","24.25","1424.6","No");
INSERT INTO churn.train VALUES("5630-AHZIL","Female","0","No","Yes","3","Yes","No","DSL","Yes","No","No","Yes","No","Yes","Month-to-month","Yes","Bank transfer (automatic)","64.5","177.4","No");
INSERT INTO churn.train VALUES("2673-CXQEU","Female","1","No","No","56","Yes","Yes","Fiber optic","Yes","Yes","Yes","No","Yes","Yes","One year","No","Electronic check","110.5","6139.5","No");
INSERT INTO churn.train VALUES("6416-JNVRK","Female","0","No","No","46","Yes","No","DSL","No","No","No","No","No","Yes","One year","No","Credit card (automatic)","55.65","2688.85","No");
INSERT INTO churn.train VALUES("5590-ZSKRV","Female","0","Yes","Yes","8","Yes","No","DSL","Yes","Yes","No","No","No","No","Month-to-month","No","Mailed check","54.65","482.25","No");
INSERT INTO churn.train VALUES("0191-ZHSKZ","Male","1","No","No","30","Yes","No","DSL","Yes","Yes","No","No","Yes","Yes","Month-to-month","Yes","Electronic check","74.75","2111.3","No");
INSERT INTO churn.train VALUES("3887-PBQAO","Female","0","Yes","Yes","45","Yes","Yes","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","One year","Yes","Credit card (automatic)","25.9","1216.6","No");
INSERT INTO churn.train VALUES("5919-TMRGD","Female","0","No","Yes","1","Yes","No","Fiber optic","No","No","No","No","Yes","No","Month-to-month","Yes","Electronic check","79.35","79.35","Yes");
INSERT INTO churn.train VALUES("8108-UXRQN","Female","0","Yes","Yes","11","No","No phone service","DSL","Yes","No","No","No","Yes","Yes","Month-to-month","No","Electronic check","50.55","565.35","No");

DROP TABLE IF EXISTS churn.test;
CREATE TABLE  churn.test (
	customerID VARCHAR(255),
	gender VARCHAR(255),
	SeniorCitizen TINYINT,
	Partner VARCHAR(255),
	Dependents VARCHAR(255),
	tenure INT,
	PhoneService VARCHAR(255),
	MultipleLines VARCHAR(255),
	InternetService VARCHAR(255),
	OnlineSecurity VARCHAR(255),
	OnlineBackup VARCHAR(255),
	DeviceProtection VARCHAR(255),
	TechSupport VARCHAR(255),
	StreamingTV VARCHAR(255),
	StreamingMovies VARCHAR(255),
	Contract VARCHAR(255),
	PaperlessBilling VARCHAR(255),
	PaymentMethod VARCHAR(255),
	MonthlyCharges FLOAT,
	TotalCharges FLOAT,
	Churn VARCHAR(255),
	PRIMARY KEY ( customerID )
);
INSERT INTO churn.test VALUES("9191-MYQKX","Female","0","Yes","No","7","Yes","No","Fiber optic","No","No","Yes","No","No","No","Month-to-month","Yes","Bank transfer (automatic)","75.15","496.9","Yes");
INSERT INTO churn.test VALUES("9919-YLNNG","Female","0","No","No","42","Yes","No","Fiber optic","No","Yes","Yes","Yes","Yes","Yes","Month-to-month","Yes","Bank transfer (automatic)","103.8","4327.5","No");
INSERT INTO churn.test VALUES("0318-ZOPWS","Female","0","Yes","No","49","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Two year","Yes","Bank transfer (automatic)","20.15","973.35","No");
INSERT INTO churn.test VALUES("4445-ZJNMU","Male","0","No","No","9","Yes","Yes","Fiber optic","No","Yes","No","No","Yes","Yes","Month-to-month","Yes","Credit card (automatic)","99.3","918.75","No");
INSERT INTO churn.test VALUES("4808-YNLEU","Female","0","Yes","No","35","Yes","No","DSL","Yes","No","No","No","Yes","No","One year","Yes","Bank transfer (automatic)","62.15","2215.45","No");
INSERT INTO churn.test VALUES("1862-QRWPE","Female","0","Yes","Yes","48","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Two year","No","Bank transfer (automatic)","20.65","1057","No");
INSERT INTO churn.test VALUES("2796-NNUFI","Female","0","Yes","Yes","46","Yes","No","No","No internet service","No internet service","No internet service","No internet service","No internet service","No internet service","Two year","Yes","Mailed check","19.95","927.1","No");
INSERT INTO churn.test VALUES("3016-KSVCP","Male","0","Yes","No","29","No","No phone service","DSL","No","No","No","No","Yes","No","Month-to-month","No","Mailed check","33.75","1009.25","No");
INSERT INTO churn.test VALUES("4767-HZZHQ","Male","0","Yes","Yes","30","Yes","No","Fiber optic","No","Yes","Yes","No","No","No","Month-to-month","No","Bank transfer (automatic)","82.05","2570.2","No");
INSERT INTO churn.test VALUES("2424-WVHPL","Male","1","No","No","1","Yes","No","Fiber optic","No","No","No","Yes","No","No","Month-to-month","No","Electronic check","74.7","74.7","No");
