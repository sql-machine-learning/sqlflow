package org.sqlflow.parser;

import java.io.File;
import java.io.FileFilter;
import java.net.URL;
import java.net.URLClassLoader;
import java.util.Enumeration;
import java.util.HashMap;
import java.util.jar.JarEntry;
import java.util.jar.JarFile;
import org.sqlflow.parser.parse.ParseInterface;

class ParserFactory {
  private HashMap<String, Class<?>> parsers;

  ParserFactory(String folderPath) throws Exception {
    parsers = new HashMap<String, Class<?>>();
    File folder = new File(folderPath);
    FileFilter sizeFilter =
        new FileFilter() {
          @Override
          public boolean accept(File file) {
            return file.getName().endsWith(".jar");
          }
        };
    File[] files = folder.listFiles(sizeFilter);

    for (File file : files) {
      String pathToJar = file.getAbsolutePath();
      URL[] urls = {new URL("jar:file:" + pathToJar + "!/")};
      ClassLoader parent = Thread.currentThread().getContextClassLoader();
      URLClassLoader cl = URLClassLoader.newInstance(urls, parent);
      // make classes loaded visible to current thread
      Thread.currentThread().setContextClassLoader(cl);

      try (JarFile jarFile = new JarFile(pathToJar)) {
        Enumeration<JarEntry> e = jarFile.entries();
        while (e.hasMoreElements()) {
          JarEntry je = e.nextElement();
          if (je.isDirectory() || !je.getName().endsWith(".class")) {
            continue;
          }
          // -6 because of .class
          String className = je.getName().substring(0, je.getName().length() - 6);
          className = className.replace('/', '.');

          if (className.startsWith("org.sqlflow")
              && !className.startsWith("org.sqlflow.parser.parse")) {
            Class<?> c = cl.loadClass(className);
            if (ParseInterface.class.isAssignableFrom(c)) {
              Object inst = c.getConstructor().newInstance();
              ParseInterface parser = (ParseInterface) inst;
              System.err.printf("ParserFactory loading class %s\n", className);
              parsers.put(parser.dialect(), c);
              break;
            }
          }
        }
      }
    }
  }

  Class<?> getAlternativeParsers(String... alts) {
    for (String alt : alts) {
      if (parsers.containsKey(alt)) {
        return parsers.get(alt);
      }
    }
    return null;
  }

  public ParseInterface newParser(String dialect) throws Exception {
    Class<?> c = null;
    if ("maxcompute".equals(dialect)) {
      // for maxcompute: chose odps or calcite according
      // to which exists in classpath
      c = this.getAlternativeParsers(dialect, "odps", "calcite");
    } else {
      c = this.getAlternativeParsers(dialect);
    }
    if (c == null) {
      throw new Exception("parser \"" + dialect + "\" not found");
    }
    return (ParseInterface) c.getConstructor().newInstance();
  }
}
