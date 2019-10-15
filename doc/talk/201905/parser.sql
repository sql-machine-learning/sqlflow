SELECT          ...
FROM            ( 
                         SELECT *
                         FROM some_table1) a 
LEFT JOIN 
                ( 
                       SELECT * 
                       FROM   some_table2
                       WHERE  ...
                       AND    ...) b 
ON              a.id = b.eventid 
LEFT OUTER JOIN 
                ( 
                       SELECT * 
                       FROM   some_table3) c 
ON              a.id = c.event_id 
GROUP BY        ...
TO TRAIN LogisticRegression
COLUMN *
LABEL score
INTO my_project.my_lr_model;
